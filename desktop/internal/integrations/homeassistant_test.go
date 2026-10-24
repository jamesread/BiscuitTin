package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jamesread/biscuittin/internal/config"
)

func fakeHomeAssistant(t *testing.T, triggered *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/states":
			_, _ = w.Write([]byte(`[
				{"entity_id": "light.tv", "state": "on", "attributes": {"friendly_name": "TV lamp"}},
				{"entity_id": "automation.tv_lighting", "state": "on", "attributes": {"friendly_name": "TV lighting", "icon": "mdi:television-ambient-light"}},
				{"entity_id": "automation.bins", "state": "off", "attributes": {"friendly_name": "Bins reminder"}}
			]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/services/automation/trigger":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			*triggered = body["entity_id"]
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestHomeAssistantListAndRun(t *testing.T) {
	var triggered string
	srv := fakeHomeAssistant(t, &triggered)
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("secret-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := newHomeAssistantSource(config.HomeAssistantConfig{URL: srv.URL, TokenFile: tokenFile})

	acts, err := src.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 2 || acts[0].Title != "Bins reminder" || acts[1].Title != "TV lighting" {
		t.Fatalf("want the two automations sorted by name, got %+v", acts)
	}
	tv := acts[1]
	if tv.BindingID != "automation.tv_lighting" || tv.Icon != "mdi:television-ambient-light" || tv.SourceID() != "homeassistant" {
		t.Errorf("TV lighting = %+v", tv)
	}
	if acts[0].Icon != "mdi:robot" || !strings.Contains(acts[0].Group, "disabled") {
		t.Errorf("automation without icon / disabled = %+v", acts[0])
	}

	events := make(chan Event, 1)
	src.Run(tv.PinnedAction, events)
	ev := <-events
	if triggered != "automation.tv_lighting" || !ev.Done || ev.Failed || ev.Key != tv.Key() {
		t.Errorf("Run: triggered %q, event %+v", triggered, ev)
	}
}

func TestHomeAssistantMissingToken(t *testing.T) {
	src := newHomeAssistantSource(config.HomeAssistantConfig{URL: "http://127.0.0.1:1", TokenFile: filepath.Join(t.TempDir(), "nope")})
	_, err := src.List()
	if err == nil || !strings.Contains(err.Error(), "no access token") {
		t.Fatalf("want a clear missing-token error, got %v", err)
	}
}

func TestHomeAssistantTokenFileFollowsConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/home/someone/.var/app/io.github.jamesread.biscuittin/config")
	src := newHomeAssistantSource(config.HomeAssistantConfig{URL: "http://127.0.0.1:1"})
	if want := "/home/someone/.var/app/io.github.jamesread.biscuittin/config/biscuittin/homeassistant.token"; src.tokenFile != want {
		t.Errorf("tokenFile = %q; want %q", src.tokenFile, want)
	}
}
