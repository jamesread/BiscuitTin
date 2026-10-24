package debugctl

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
)

func TestSocketPathUsesRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got, want := SocketPath(), "/run/user/1000/biscuittin/debug.sock"; got != want {
		t.Fatalf("SocketPath() = %q; want %q", got, want)
	}
}

func TestSocketPathFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	got := SocketPath()
	want := filepath.Join(os.TempDir(), "biscuittin-"+strconv.Itoa(os.Getuid()), "debug.sock")
	if got != want {
		t.Fatalf("SocketPath() = %q; want %q", got, want)
	}
}

func TestGetDiagnosticsOverSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "debug.sock")
	bridge := NewBridge()
	bridge.Publish(&pb.GetDiagnosticsResponse{
		Version:    "dev",
		SocketPath: path,
		Window:     &pb.Window{Width: 2160, Height: 3840, Fullscreen: true},
	})
	srv, err := Start(path, bridge)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	if srv.Path() != path {
		t.Fatalf("Path() = %q", srv.Path())
	}

	waitListening(t, path)
	resp, err := NewClient(path).GetDiagnostics(context.Background(), connect.NewRequest(&pb.GetDiagnosticsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetVersion() != "dev" || resp.Msg.GetWindow().GetWidth() != 2160 || resp.Msg.GetWindow().GetHeight() != 3840 {
		t.Fatalf("GetDiagnostics = %v", resp.Msg)
	}

	var buf bytes.Buffer
	if err := PrintDiagnosticsFrom(&buf, path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "2160") || !strings.Contains(buf.String(), "3840") {
		t.Fatalf("JSON = %s", buf.String())
	}
}

func TestStartReplacesStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()

	srv, err := Start(path, NewBridge())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	waitListening(t, path)
}

func TestStartLeavesLiveInstance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.sock")
	first, err := Start(path, NewBridge())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Stop)
	waitListening(t, path)

	if _, err := Start(path, NewBridge()); err == nil {
		t.Fatal("second Start should fail while the first is live")
	}
	if _, err := NewClient(path).GetDiagnostics(context.Background(), connect.NewRequest(&pb.GetDiagnosticsRequest{})); err != nil {
		t.Fatalf("first instance stopped answering: %v", err)
	}
}

func TestPrintDiagnosticsMissingInstance(t *testing.T) {
	err := PrintDiagnosticsFrom(&bytes.Buffer{}, filepath.Join(t.TempDir(), "missing.sock"))
	if err == nil || !strings.Contains(err.Error(), "no running instance") {
		t.Fatalf("err = %v", err)
	}
}

func waitListening(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		d := &net.Dialer{Timeout: 50 * time.Millisecond}
		conn, err := d.DialContext(context.Background(), "unix", path)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket not listening: %s", path)
}
