// Package webcontrol serves the phone web remote and its ConnectRPC API.
// It only runs once switched on in Settings (off by default).
package webcontrol

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/gen/biscuittin/v1/biscuittinv1connect"
	"github.com/jamesread/biscuittin/internal/config"
)

// The remote-control page, built from frontend/ (at the repo root) into
// webdist/ by `make`. Only a .gitkeep is committed, so `go build` without the
// frontend still compiles; the server then answers with a hint to build it.
//
//go:embed all:webdist
var webDist embed.FS

// defaultWebPort is used when neither $PORT nor webControl.port is set.
const defaultWebPort = 8137

func Port(cfg config.WebControlConfig) int {
	if p, err := strconv.Atoi(os.Getenv("PORT")); err == nil && p > 0 {
		return p
	}
	if cfg.Port > 0 {
		return cfg.Port
	}
	return defaultWebPort
}

// Command is input from the web remote, applied on the main thread.
// Exactly one of the fields is set.
type Command struct {
	Button  pb.Button
	TileKey string
}

// Bridge connects the HTTP handlers (on their own goroutines) to the
// single-threaded UI: the main loop publishes a state snapshot each frame and
// drains commands.
type Bridge struct {
	mu       sync.Mutex
	state    *pb.GetStateResponse
	Commands chan Command
}

func NewBridge() *Bridge {
	return &Bridge{
		state:    &pb.GetStateResponse{},
		Commands: make(chan Command, 32),
	}
}

func (b *Bridge) Publish(s *pb.GetStateResponse) {
	b.mu.Lock()
	b.state = s
	b.mu.Unlock()
}

func (b *Bridge) snapshot() *pb.GetStateResponse {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// send queues a command, dropping it if the UI has fallen far behind rather
// than blocking the HTTP request.
func (b *Bridge) send(c Command) error {
	select {
	case b.Commands <- c:
		return nil
	default:
		return connect.NewError(connect.CodeResourceExhausted, errors.New("too many queued inputs"))
	}
}

// remoteService implements biscuittin.v1.RemoteService.
type remoteService struct {
	bridge *Bridge
}

func (r *remoteService) GetState(context.Context, *connect.Request[pb.GetStateRequest]) (*connect.Response[pb.GetStateResponse], error) {
	return connect.NewResponse(r.bridge.snapshot()), nil
}

func (r *remoteService) PressButton(_ context.Context, req *connect.Request[pb.PressButtonRequest]) (*connect.Response[pb.PressButtonResponse], error) {
	b := req.Msg.GetButton()
	if b == pb.Button_BUTTON_UNSPECIFIED || pb.Button_name[int32(b)] == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown button %v", b))
	}
	if err := r.bridge.send(Command{Button: b}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.PressButtonResponse{}), nil
}

func (r *remoteService) ActivateTile(_ context.Context, req *connect.Request[pb.ActivateTileRequest]) (*connect.Response[pb.ActivateTileResponse], error) {
	key := req.Msg.GetKey()
	if !r.knownTile(key) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no tile %q", key))
	}
	if err := r.bridge.send(Command{TileKey: key}); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ActivateTileResponse{}), nil
}

func (r *remoteService) knownTile(key string) bool {
	for _, t := range r.bridge.snapshot().GetTiles() {
		if t.GetKey() == key {
			return true
		}
	}
	return false
}

func newWebControlMux(bridge *Bridge) http.Handler {
	mux := http.NewServeMux()
	path, handler := biscuittinv1connect.NewRemoteServiceHandler(&remoteService{bridge: bridge})
	mux.Handle(path, handler)

	dist, err := fs.Sub(webDist, "webdist")
	if err == nil {
		if _, statErr := fs.Stat(dist, "index.html"); statErr == nil {
			mux.Handle("/", http.FileServer(http.FS(dist)))
			return mux
		}
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "biscuittin was built without the web remote; run `make` to build frontend/", http.StatusNotFound)
	})
	return mux
}

// Server is the running web server; it is started and stopped as the
// setting changes.
type Server struct {
	srv  *http.Server
	port int
}

func Start(port int, bridge *Bridge) (*Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}
	wc := &Server{
		srv:  &http.Server{Handler: newWebControlMux(bridge), ReadHeaderTimeout: 10 * time.Second},
		port: port,
	}
	go func() {
		if err := wc.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lg().Error("web control server", zap.Error(err))
		}
	}()
	lg().Info("web control listening", zap.String("addr", fmt.Sprintf("0.0.0.0:%d", port)), zap.String("url", wc.URL()))
	return wc, nil
}

func (wc *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = wc.srv.Shutdown(ctx)
	lg().Info("web control stopped")
}

// URL is the address a phone on the LAN should open.
func (wc *Server) URL() string {
	return fmt.Sprintf("http://%s:%d/", lanAddress(), wc.port)
}

// lanAddress finds the IPv4 address used for the default route. Dialling UDP
// sends no packets; it only selects a source address.
func lanAddress() string {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && !addr.IP.IsLoopback() {
			return addr.IP.String()
		}
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "localhost"
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("webcontrol") }
