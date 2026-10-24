package icons

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// iconifyProvider fetches icons from the Iconify API by "prefix:name", e.g.
// "material-symbols:login". OliveTin actions name their icons this way.
type iconifyProvider struct {
	client *http.Client
}

var iconifyQuery = regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)

func newIconifyProvider() *iconifyProvider {
	return &iconifyProvider{client: &http.Client{Timeout: 20 * time.Second}}
}

func (p *iconifyProvider) Name() string { return "iconify" }

func readIconifyBody(url string, resp *http.Response) ([]byte, error) {
	if resp.StatusCode == http.StatusNotFound {
		return nil, errIconNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

func (p *iconifyProvider) getIcon(url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return readIconifyBody(url, resp)
}

func (p *iconifyProvider) Find(query, _ string) ([]byte, error) {
	if !iconifyQuery.MatchString(query) {
		return nil, errIconNotFound
	}
	prefix, name, _ := strings.Cut(query, ":")
	// Ask for a solid colour; Iconify SVGs otherwise use currentColor, which
	// the rasteriser does not resolve. Mono icons are tinted white afterwards.
	url := fmt.Sprintf("https://api.iconify.design/%s/%s.svg?color=%%23000000", prefix, name)
	raw, err := p.getIcon(url)
	if err != nil {
		return nil, err
	}
	// Iconify answers unknown icons with 404, but guard against non-SVG bodies.
	if !strings.Contains(string(raw), "<svg") {
		return nil, errIconNotFound
	}
	return raw, nil
}
