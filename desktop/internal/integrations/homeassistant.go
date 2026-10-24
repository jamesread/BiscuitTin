package integrations

import (
	"bytes"
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
	base      string
	token     string
	tokenFile string
	http      *http.Client
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

func (s *homeAssistantSource) do(method, path string, body any, out any) error {
	token, err := s.bearer()
	if err != nil {
		return err
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, s.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("access token rejected (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
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
func (s *homeAssistantSource) List() ([]ListedAction, error) {
	var states []haState
	if err := s.do(http.MethodGet, "/api/states", nil, &states); err != nil {
		return nil, err
	}

	var out []ListedAction
	for _, st := range states {
		if !strings.HasPrefix(st.EntityID, "automation.") {
			continue
		}
		title := st.Attributes.FriendlyName
		if title == "" {
			title = st.EntityID
		}
		icon := iconifyName(st.Attributes.Icon) // HA icons are "mdi:name", also on Iconify
		if icon == "" {
			icon = "mdi:robot"
		}
		group := "Automation"
		if st.State == "off" {
			group = "Automation  /  disabled"
		}
		out = append(out, ListedAction{PinnedAction: PinnedAction{
			Source:    s.ID(),
			BindingID: st.EntityID,
			Title:     title,
			Group:     group,
			Icon:      icon,
		}})
	}
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
