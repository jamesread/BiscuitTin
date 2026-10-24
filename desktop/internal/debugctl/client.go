package debugctl

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/jamesread/biscuittin/gen/biscuittin/v1"
	"github.com/jamesread/biscuittin/gen/biscuittin/v1/biscuittinv1connect"
)

// PrintDiagnostics writes the running instance's diagnostics as JSON to w.
func PrintDiagnostics(w io.Writer) error {
	return PrintDiagnosticsFrom(w, SocketPath())
}

// PrintDiagnosticsFrom is PrintDiagnostics for a given socket path.
func PrintDiagnosticsFrom(w io.Writer, socketPath string) error {
	if _, err := os.Stat(socketPath); err != nil {
		return fmt.Errorf("no running instance (%s)", socketPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := NewClient(socketPath).GetDiagnostics(ctx, connect.NewRequest(&pb.GetDiagnosticsRequest{}))
	if err != nil {
		return err
	}
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(resp.Msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}

// NewClient talks to DebugService over a unix socket.
func NewClient(socketPath string) biscuittinv1connect.DebugServiceClient {
	t := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return biscuittinv1connect.NewDebugServiceClient(&http.Client{Transport: t}, "http://localhost")
}
