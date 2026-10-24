package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func fakeOliveTin(t *testing.T, started *map[string]any) *httptest.Server {
	t.Helper()
	dashboards := map[string]any{
		"Actions": map[string]any{"dashboard": map[string]any{"title": "Actions", "contents": []any{
			map[string]any{"title": "Restart", "type": "link", "action": map[string]any{
				"bindingId": "restart", "title": "Restart", "canExec": true,
				"icon": `<iconify-icon icon="material-symbols:restart-alt"></iconify-icon>`,
			}},
			map[string]any{"title": "Ping", "type": "link", "action": map[string]any{
				"bindingId": "ping", "title": "Ping", "canExec": true,
				"arguments": []any{map[string]any{"name": "host", "type": "ascii", "defaultValue": ""}},
			}},
			map[string]any{"title": "Secret", "type": "link", "action": map[string]any{
				"bindingId": "secret", "title": "Secret", "canExec": false,
			}},
		}}},
		"SOE": map[string]any{"dashboard": map[string]any{"title": "SOE", "contents": []any{
			map[string]any{"title": "queen (10.0.0.2)", "type": "fieldset", "contents": []any{
				map[string]any{"title": "Deploy", "type": "link", "action": map[string]any{
					"bindingId": "abc123", "title": "Deploy", "canExec": true,
					"arguments": []any{map[string]any{"name": "ansible_host", "defaultValue": "queen"}},
				}},
				// Same binding listed twice must only appear once.
				map[string]any{"title": "Restart", "type": "link", "action": map[string]any{
					"bindingId": "restart", "title": "Restart", "canExec": true,
				}},
			}},
		}}},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch r.URL.Path {
		case "/api/Init":
			_ = json.NewEncoder(w).Encode(map[string]any{"rootDashboards": []string{"Actions", "SOE"}})
		case "/api/GetDashboard":
			_ = json.NewEncoder(w).Encode(dashboards[req["title"].(string)])
		case "/api/StartAction":
			*started = req
			_ = json.NewEncoder(w).Encode(map[string]any{"executionTrackingId": "exec-1"})
		case "/api/ExecutionStatus":
			_ = json.NewEncoder(w).Encode(map[string]any{"logEntry": map[string]any{"executionFinished": true, "exitCode": 3}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestOliveTinListActions(t *testing.T) {
	var started map[string]any
	srv := fakeOliveTin(t, &started)
	defer srv.Close()

	acts, err := newOliveTinClient(srv.URL + "/").ListActions()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ListedAction{}
	for _, a := range acts {
		byID[a.BindingID] = a
	}

	if len(acts) != 3 {
		t.Fatalf("want 3 runnable, de-duplicated actions, got %d: %+v", len(acts), acts)
	}
	if _, ok := byID["secret"]; ok {
		t.Error("actions the user cannot exec should be skipped")
	}
	if a := byID["restart"]; a.Icon != "material-symbols:restart-alt" || a.Group != "Actions" {
		t.Errorf("restart = %+v; want iconify name and its dashboard as group", a)
	}
	if a := byID["abc123"]; a.Group != "queen (10.0.0.2)" || a.NeedsInput || len(a.Args) != 1 || a.Args[0].Value != "queen" {
		t.Errorf("entity action = %+v; want fieldset group and default argument", a)
	}
	if !byID["ping"].NeedsInput {
		t.Error("an argument without a default should mark the action as needing input")
	}
}

func TestOliveTinStartAndWait(t *testing.T) {
	var started map[string]any
	srv := fakeOliveTin(t, &started)
	defer srv.Close()

	c := newOliveTinClient(srv.URL)
	id, err := c.Start(PinnedAction{BindingID: "abc123", Args: []ArgValue{{Name: "ansible_host", Value: "queen"}}})
	if err != nil || id != "exec-1" {
		t.Fatalf("Start = %q, %v", id, err)
	}
	if started["bindingId"] != "abc123" {
		t.Errorf("StartAction request = %v", started)
	}
	code, err := c.Wait(id, 5*time.Second)
	if err != nil || code != 3 {
		t.Errorf("Wait = %d, %v; want exit 3", code, err)
	}
}

func TestIconifyName(t *testing.T) {
	if got := iconifyName(`<iconify-icon icon="simple-icons:fedora"></iconify-icon>`); got != "simple-icons:fedora" {
		t.Errorf("got %q", got)
	}
	if got := iconifyName("hugeicons:CommandLineIcon"); got != "hugeicons:command-line" {
		t.Errorf("component-style name: got %q", got)
	}
	if got := iconifyName("&#x1F4BE;"); got != "" {
		t.Errorf("emoji icon should give no iconify name, got %q", got)
	}
}
