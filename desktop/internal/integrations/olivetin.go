package integrations

import (
	"bytes"
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
	base string
	http *http.Client
}

func newOliveTinClient(url string) *olivetinClient {
	return &olivetinClient{
		base: strings.TrimRight(url, "/"),
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *olivetinClient) call(method string, req, resp any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	r, err := c.http.Post(c.base+"/api/"+method, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s %s", method, r.Status, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, resp)
}

type otNode struct {
	Title    string    `json:"title"`
	Type     string    `json:"type"`
	Contents []otNode  `json:"contents"`
	Action   *otRawAct `json:"action"`
}

type otRawAct struct {
	BindingID string `json:"bindingId"`
	Title     string `json:"title"`
	Icon      string `json:"icon"`
	CanExec   bool   `json:"canExec"`
	Arguments []struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		DefaultValue string `json:"defaultValue"`
	} `json:"arguments"`
}

// ListActions walks every root dashboard and returns the actions the current
// user can run, labelled with the dashboard or entity group they sit in.
// OliveTin has no single "list actions" call, so this mirrors what its web UI
// shows.
func (c *olivetinClient) ListActions() ([]ListedAction, error) {
	var init struct {
		RootDashboards []string `json:"rootDashboards"`
	}
	if err := c.call("Init", struct{}{}, &init); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var out []ListedAction

	var walk func(n otNode, group string)
	walk = func(n otNode, group string) {
		if n.Type == "fieldset" && n.Title != "" {
			group = n.Title
		}
		if a := n.Action; a != nil && a.BindingID != "" && a.CanExec && !seen[a.BindingID] {
			seen[a.BindingID] = true
			act := ListedAction{PinnedAction: PinnedAction{
				BindingID: a.BindingID,
				Title:     a.Title,
				Group:     group,
				Icon:      iconifyName(a.Icon),
			}}
			for _, arg := range a.Arguments {
				if arg.Type == "confirmation" {
					act.Args = append(act.Args, ArgValue{Name: arg.Name, Value: "1"})
					continue
				}
				if arg.DefaultValue == "" {
					act.NeedsInput = true
				}
				act.Args = append(act.Args, ArgValue{Name: arg.Name, Value: arg.DefaultValue})
			}
			out = append(out, act)
		}
		for _, child := range n.Contents {
			walk(child, group)
		}
	}

	for _, title := range init.RootDashboards {
		var resp struct {
			Dashboard otNode `json:"dashboard"`
		}
		if err := c.call("GetDashboard", map[string]string{"title": title}, &resp); err != nil {
			return nil, fmt.Errorf("dashboard %q: %w", title, err)
		}
		walk(resp.Dashboard, title)
	}
	return out, nil
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
