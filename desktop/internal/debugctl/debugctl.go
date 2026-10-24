// Package debugctl serves a local, read-only inspect API on a unix socket.
package debugctl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/gen/biscuittin/v1/biscuittinv1connect"
)

const socketName = "debug.sock"

// SocketPath is $XDG_RUNTIME_DIR/biscuittin/debug.sock, or
// $TMPDIR/biscuittin-$UID/debug.sock when XDG_RUNTIME_DIR is unset.
func SocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), fmt.Sprintf("biscuittin-%d", os.Getuid()))
	} else {
		dir = filepath.Join(dir, "biscuittin")
	}
	return filepath.Join(dir, socketName)
}

// Bridge holds the latest diagnostics snapshot, published from the main loop.
type Bridge struct {
	mu    sync.Mutex
	state *pb.GetDiagnosticsResponse
}

func NewBridge() *Bridge {
	return &Bridge{state: &pb.GetDiagnosticsResponse{}}
}

func (b *Bridge) Publish(s *pb.GetDiagnosticsResponse) {
	b.mu.Lock()
	b.state = s
	b.mu.Unlock()
}

func (b *Bridge) snapshot() *pb.GetDiagnosticsResponse {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

type debugService struct {
	bridge *Bridge
}

func (d *debugService) GetDiagnostics(context.Context, *connect.Request[pb.GetDiagnosticsRequest]) (*connect.Response[pb.GetDiagnosticsResponse], error) {
	return connect.NewResponse(d.bridge.snapshot()), nil
}

// Server is the running debug socket.
type Server struct {
	srv  *http.Server
	ln   net.Listener
	path string
}

// Start listens on path (or SocketPath if path is empty). A stale socket left
// by a crashed process is replaced. A live instance returns an error; the
// caller should log and continue.
func Start(path string, bridge *Bridge) (*Server, error) {
	if path == "" {
		path = SocketPath()
	}
	ln, err := listenUnix(path)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	p, handler := biscuittinv1connect.NewDebugServiceHandler(&debugService{bridge: bridge})
	mux.Handle(p, handler)
	s := &Server{
		srv:  &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second},
		ln:   ln,
		path: path,
	}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lg().Error("debug server", zap.Error(err))
		}
	}()
	lg().Info("listening", zap.String("path", path))
	return s, nil
}

func (s *Server) Path() string { return s.path }

func (s *Server) Stop() {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
	_ = os.Remove(s.path)
	lg().Info("stopped", zap.String("path", s.path))
}

func listenUnix(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err == nil {
		_ = os.Chmod(path, 0o600)
		return ln, nil
	}
	conn, dialErr := net.DialTimeout("unix", path, 200*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return nil, fmt.Errorf("already running (%s)", path)
	}
	lg().Info("removed stale socket", zap.String("path", path))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ln, err = net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return ln, nil
}

func lg() *zap.Logger { return zap.L().Named("debug") }
