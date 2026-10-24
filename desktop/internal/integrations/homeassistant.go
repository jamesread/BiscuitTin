package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"go.uber.org/zap"
)

// defaultHATokenFile is next to the config file: normally
// ~/.config/biscuittin/homeassistant.token, or inside the app's own config
// directory under Flatpak.
func defaultHATokenFile() string {
	return filepath.Join(filepath.Dir(config.DefaultPath()), "homeassistant.token")
}

type homeAssistantSource struct {
	http      *http.Client
	base      string
	token     string
	tokenFile string
}

func newHomeAssistantSource(c config.HomeAssistantConfig) *homeAssistantSource {
	s := &homeAssistantSource{
		base:      strings.TrimRight(c.URL, "/"),
		token:     strings.TrimSpace(c.Token),
		tokenFile: c.TokenFile,
		http:      &http.Client{Timeout: 20 * time.Second},
	}
	if s.tokenFile == "" {
		s.tokenFile = defaultHATokenFile()
	}
	s.tokenFile = config.ExpandPath(s.tokenFile)
	return s
}

func (s *homeAssistantSource) ID() string    { return "homeassistant" }
func (s *homeAssistantSource) Title() string { return "Home Assistant automations" }
func (s *homeAssistantSource) Where() string { return s.base }

// bearer reads the token on each use, so a token file added later works
// without restarting.
func (s *homeAssistantSource) bearer() (string, error) {
	if s.token != "" {
		return s.token, nil
	}
	raw, err := os.ReadFile(s.tokenFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no access token: put a long-lived token in %s", s.tokenFile)
		}
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

type haResp struct {
	status string
	raw    []byte
	code   int
}

func jsonReader(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(raw), nil
}

func readHTTP(client *http.Client, req *http.Request) (haResp, error) {
	resp, err := client.Do(req)
	if err != nil {
		return haResp{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return haResp{}, err
	}
	return haResp{raw: raw, status: resp.Status, code: resp.StatusCode}, nil
}

func (s *homeAssistantSource) haRequest(method, path, token string, body any) (haResp, error) {
	rd, err := jsonReader(body)
	if err != nil {
		return haResp{}, err
	}
	req, err := http.NewRequestWithContext(context.Background(), method, s.base+path, rd)
	if err != nil {
		return haResp{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return readHTTP(s.http, req)
}

func decodeHA(method, path string, res haResp, out any) error {
	if res.code == http.StatusUnauthorized {
		return fmt.Errorf("access token rejected (401)")
	}
	if res.code != http.StatusOK {
		return fmt.Errorf("%s %s: %s", method, path, res.status)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(res.raw, out)
}

func (s *homeAssistantSource) do(method, path string, body any, out any) error {
	token, err := s.bearer()
	if err != nil {
		return err
	}
	res, err := s.haRequest(method, path, token, body)
	if err != nil {
		return err
	}
	return decodeHA(method, path, res, out)
}

type haState struct {
	EntityID   string `json:"entity_id"`
	State      string `json:"state"`
	Attributes struct {
		FriendlyName string `json:"friendly_name"`
		Icon         string `json:"icon"`
	} `json:"attributes"`
}

// List returns every automation, sorted by name.
func haTitle(st haState) string {
	if st.Attributes.FriendlyName != "" {
		return st.Attributes.FriendlyName
	}
	return st.EntityID
}

func haIcon(icon string) string {
	if name := iconifyName(icon); name != "" { // HA icons are "mdi:name", also on Iconify
		return name
	}
	return "mdi:robot"
}

func haGroup(state string) string {
	if state == "off" {
		return "Automation  /  disabled"
	}
	return "Automation"
}

func listedAutomation(source string, st haState) (ListedAction, bool) {
	if !strings.HasPrefix(st.EntityID, "automation.") {
		return ListedAction{}, false
	}
	return ListedAction{PinnedAction: PinnedAction{
		Source:    source,
		BindingID: st.EntityID,
		Title:     haTitle(st),
		Group:     haGroup(st.State),
		Icon:      haIcon(st.Attributes.Icon),
	}}, true
}

func automationsFrom(source string, states []haState) []ListedAction {
	var out []ListedAction
	for _, st := range states {
		if a, ok := listedAutomation(source, st); ok {
			out = append(out, a)
		}
	}
	return out
}

func (s *homeAssistantSource) List() ([]ListedAction, error) {
	var states []haState
	if err := s.do(http.MethodGet, "/api/states", nil, &states); err != nil {
		return nil, err
	}
	out := automationsFrom(s.ID(), states)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

// Run triggers the automation's actions (skipping its conditions, as the
// "Run actions" button in Home Assistant does).
func (s *homeAssistantSource) Run(a PinnedAction, events chan<- Event) {
	err := s.do(http.MethodPost, "/api/services/automation/trigger", map[string]string{"entity_id": a.BindingID}, nil)
	if err != nil {
		lg("homeassistant").Warn("trigger failed", zap.String("automation", a.BindingID), zap.Error(err))
		events <- Event{Key: a.Key(), Msg: "Could not run " + a.Title + ": " + ShortErr(err), Failed: true, Done: true}
		return
	}
	lg("homeassistant").Info("triggered", zap.String("automation", a.BindingID))
	events <- Event{Key: a.Key(), Msg: "Ran " + a.Title, Done: true}
}
