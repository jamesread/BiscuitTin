package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ArgValue struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

type olivetinClient struct {
	http *http.Client
	base string
}

func newOliveTinClient(url string) *olivetinClient {
	return &olivetinClient{
		base: strings.TrimRight(url, "/"),
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

type apiResult struct {
	status string
	raw    []byte
	code   int
}

func (c *olivetinClient) call(method string, req, resp any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	res, err := c.postAPI(method, body)
	if err != nil {
		return err
	}
	if res.code != http.StatusOK {
		return fmt.Errorf("%s: %s %s", method, res.status, strings.TrimSpace(string(res.raw)))
	}
	return json.Unmarshal(res.raw, resp)
}

func (c *olivetinClient) postAPI(method string, body []byte) (apiResult, error) {
	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.base+"/api/"+method, bytes.NewReader(body))
	if err != nil {
		return apiResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	r, err := c.http.Do(httpReq)
	if err != nil {
		return apiResult{}, err
	}
	defer func() { _ = r.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		return apiResult{}, err
	}
	return apiResult{raw: raw, status: r.Status, code: r.StatusCode}, nil
}

type otNode struct {
	Action   *otRawAct `json:"action"`
	Title    string    `json:"title"`
	Type     string    `json:"type"`
	Contents []otNode  `json:"contents"`
}

type otArg struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"defaultValue"`
}

type otRawAct struct {
	BindingID string  `json:"bindingId"`
	Title     string  `json:"title"`
	Icon      string  `json:"icon"`
	Arguments []otArg `json:"arguments"`
	CanExec   bool    `json:"canExec"`
}

type otCollector struct {
	seen map[string]bool
	out  []ListedAction
}

// ListActions walks every root dashboard and returns the actions the current
// user can run, labelled with the dashboard or entity group they sit in.
// OliveTin has no single "list actions" call, so this mirrors what its web UI
// shows.
func (c *olivetinClient) ListActions() ([]ListedAction, error) {
	roots, err := c.rootDashboards()
	if err != nil {
		return nil, err
	}
	col := &otCollector{seen: map[string]bool{}}
	for _, title := range roots {
		if err := c.collectDashboard(col, title); err != nil {
			return nil, err
		}
	}
	return col.out, nil
}

func (c *olivetinClient) rootDashboards() ([]string, error) {
	var init struct {
		RootDashboards []string `json:"rootDashboards"`
	}
	if err := c.call("Init", struct{}{}, &init); err != nil {
		return nil, err
	}
	return init.RootDashboards, nil
}

func (c *olivetinClient) collectDashboard(col *otCollector, title string) error {
	var resp struct {
		Dashboard otNode `json:"dashboard"`
	}
	if err := c.call("GetDashboard", map[string]string{"title": title}, &resp); err != nil {
		return fmt.Errorf("dashboard %q: %w", title, err)
	}
	col.walk(resp.Dashboard, title)
	return nil
}

func (col *otCollector) walk(n otNode, group string) {
	group = fieldsetGroup(n, group)
	col.collectAction(n, group)
	for _, child := range n.Contents {
		col.walk(child, group)
	}
}

func fieldsetGroup(n otNode, group string) string {
	if n.Type == "fieldset" && n.Title != "" {
		return n.Title
	}
	return group
}

func runnableAction(a *otRawAct, seen map[string]bool) bool {
	if a == nil {
		return false
	}
	if a.BindingID == "" {
		return false
	}
	if !a.CanExec {
		return false
	}
	return !seen[a.BindingID]
}

func (col *otCollector) collectAction(n otNode, group string) {
	a := n.Action
	if !runnableAction(a, col.seen) {
		return
	}
	col.seen[a.BindingID] = true
	col.out = append(col.out, listedFromRaw(a, group))
}

func listedFromRaw(a *otRawAct, group string) ListedAction {
	act := ListedAction{PinnedAction: PinnedAction{
		BindingID: a.BindingID,
		Title:     a.Title,
		Group:     group,
		Icon:      iconifyName(a.Icon),
	}}
	for _, arg := range a.Arguments {
		act = appendOTArg(act, arg)
	}
	return act
}

func appendOTArg(act ListedAction, arg otArg) ListedAction {
	if arg.Type == "confirmation" {
		act.Args = append(act.Args, ArgValue{Name: arg.Name, Value: "1"})
		return act
	}
	if arg.DefaultValue == "" {
		act.NeedsInput = true
	}
	act.Args = append(act.Args, ArgValue{Name: arg.Name, Value: arg.DefaultValue})
	return act
}

// Start runs an action and returns its execution tracking ID.
func (c *olivetinClient) Start(a PinnedAction) (string, error) {
	req := struct {
		BindingID string     `json:"bindingId"`
		Arguments []ArgValue `json:"arguments"`
	}{a.BindingID, a.Args}
	if req.Arguments == nil {
		req.Arguments = []ArgValue{}
	}
	var resp struct {
		ExecutionTrackingID string `json:"executionTrackingId"`
	}
	if err := c.call("StartAction", req, &resp); err != nil {
		return "", err
	}
	return resp.ExecutionTrackingID, nil
}

// Wait polls an execution until it finishes (or timeout) and returns its exit
// code.
func (c *olivetinClient) Wait(trackingID string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var resp struct {
			LogEntry struct {
				ExecutionFinished bool  `json:"executionFinished"`
				ExitCode          int32 `json:"exitCode"`
			} `json:"logEntry"`
		}
		if err := c.call("ExecutionStatus", map[string]string{"executionTrackingId": trackingID}, &resp); err != nil {
			return 0, err
		}
		if resp.LogEntry.ExecutionFinished {
			return int(resp.LogEntry.ExitCode), nil
		}
		time.Sleep(2 * time.Second)
	}
	return 0, fmt.Errorf("still running after %s", timeout)
}

var (
	iconifyAttr  = regexp.MustCompile(`icon="([a-z0-9-]+:[A-Za-z0-9-]+)"`)
	iconifyBare  = regexp.MustCompile(`^([a-z0-9-]+):([A-Za-z0-9-]+)$`)
	camelBoundry = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

// iconifyName extracts an Iconify "prefix:name" from an OliveTin icon, which
// is either <iconify-icon icon="prefix:name"> markup or a bare "prefix:name".
// Component-style names such as "hugeicons:CommandLineIcon" are converted to
// Iconify's kebab-case ("hugeicons:command-line"). Emoji etc. give "".
func iconifyName(icon string) string {
	icon = strings.TrimSpace(icon)
	if m := iconifyAttr.FindStringSubmatch(icon); m != nil {
		icon = m[1]
	}
	m := iconifyBare.FindStringSubmatch(icon)
	if m == nil {
		return ""
	}
	name := m[2]
	if strings.ToLower(name) != name {
		name = strings.TrimSuffix(name, "Icon")
		name = strings.ToLower(camelBoundry.ReplaceAllString(name, "$1-$2"))
	}
	return m[1] + ":" + name
}
