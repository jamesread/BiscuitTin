package webcontrol

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/gen/biscuittin/v1/biscuittinv1connect"
	"github.com/jamesread/biscuittin/internal/config"
)

func newTestRemote(t *testing.T) (*Bridge, biscuittinv1connect.RemoteServiceClient, *httptest.Server) {
	t.Helper()
	bridge := NewBridge()
	bridge.Publish(&pb.GetStateResponse{
		Title:  "BiscuitTin on test",
		Screen: "home",
		Tiles:  []*pb.Tile{{Key: "entry:0", Title: "RetroArch", Selected: true}},
	})
	srv := httptest.NewServer(newWebControlMux(bridge))
	t.Cleanup(srv.Close)
	return bridge, biscuittinv1connect.NewRemoteServiceClient(http.DefaultClient, srv.URL), srv
}

func TestRemoteGetState(t *testing.T) {
	_, client, _ := newTestRemote(t)
	resp, err := client.GetState(context.Background(), connect.NewRequest(&pb.GetStateRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetTitle() != "BiscuitTin on test" || len(resp.Msg.GetTiles()) != 1 || !resp.Msg.GetTiles()[0].GetSelected() {
		t.Errorf("GetState = %v", resp.Msg)
	}
}

func TestRemotePressButtonQueuesAction(t *testing.T) {
	bridge, client, _ := newTestRemote(t)
	_, err := client.PressButton(context.Background(), connect.NewRequest(&pb.PressButtonRequest{Button: pb.Button_BUTTON_BACK}))
	if err != nil {
		t.Fatal(err)
	}
	if c := <-bridge.Commands; c.Button != pb.Button_BUTTON_BACK {
		t.Errorf("queued %+v; want Back", c)
	}

	_, err = client.PressButton(context.Background(), connect.NewRequest(&pb.PressButtonRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("unspecified button: err %v; want InvalidArgument", err)
	}
}

func TestRemoteActivateTile(t *testing.T) {
	bridge, client, _ := newTestRemote(t)

	_, err := client.ActivateTile(context.Background(), connect.NewRequest(&pb.ActivateTileRequest{Key: "nope"}))
	var cerr *connect.Error
	if !errors.As(err, &cerr) || cerr.Code() != connect.CodeNotFound {
		t.Errorf("unknown tile: err %v; want NotFound", err)
	}

	if _, err := client.ActivateTile(context.Background(), connect.NewRequest(&pb.ActivateTileRequest{Key: "entry:0"})); err != nil {
		t.Fatal(err)
	}
	if c := <-bridge.Commands; c.TileKey != "entry:0" {
		t.Errorf("queued %+v; want tile entry:0", c)
	}
}

func TestRemoteQueueIsBounded(t *testing.T) {
	bridge, client, _ := newTestRemote(t)
	for i := 0; i < cap(bridge.Commands); i++ {
		if err := bridge.send(Command{Button: pb.Button_BUTTON_UP}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := client.PressButton(context.Background(), connect.NewRequest(&pb.PressButtonRequest{Button: pb.Button_BUTTON_UP}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Errorf("full queue: err %v; want ResourceExhausted rather than blocking", err)
	}
}

func TestRemoteServesPage(t *testing.T) {
	_, _, srv := newTestRemote(t)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	// Built frontend: the page. Without `make`: a clear hint, not a crash.
	if !strings.Contains(string(body), "BiscuitTin remote") && !strings.Contains(string(body), "run `make`") {
		t.Errorf("GET / = %d %q", resp.StatusCode, body)
	}
}

func TestListenAddrUsesPort(t *testing.T) {
	t.Setenv("PORT", "")
	got := ListenAddr(config.WebControlConfig{Port: 9000})
	if !strings.HasSuffix(got, ":9000") {
		t.Fatalf("ListenAddr = %q", got)
	}
}

func TestWebControlPort(t *testing.T) {
	t.Setenv("PORT", "")
	if p := Port(config.WebControlConfig{}); p != defaultWebPort {
		t.Errorf("default port = %d", p)
	}
	if p := Port(config.WebControlConfig{Port: 9000}); p != 9000 {
		t.Errorf("configured port = %d", p)
	}
	t.Setenv("PORT", "9100")
	if p := Port(config.WebControlConfig{Port: 9000}); p != 9100 {
		t.Errorf("$PORT should win, got %d", p)
	}
}
