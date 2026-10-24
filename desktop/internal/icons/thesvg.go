package icons

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/jamesread/biscuittin/internal/fsutil"
)

const (
	theSVGBase        = "https://thesvg.org"
	theSVGRegistryTTL = 7 * 24 * time.Hour
	theSVGMaxRegistry = 32 << 20
	theSVGMaxIcon     = 2 << 20
)

// theSVGProvider searches https://thesvg.org. The site is static with no
// search endpoint: it publishes a full manifest (/api/registry.json) that is
// cached locally and matched client-side, and serves icons at
// /icons/{slug}/{variant}.svg.
type theSVGProvider struct {
	err      error
	client   *http.Client
	cacheDir string
	icons    []theSVGIcon
	once     sync.Once
}

type theSVGIcon struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Aliases  []string `json:"aliases"`
	Variants []string `json:"variants"`
}

func newTheSVGProvider(cacheDir string) *theSVGProvider {
	return &theSVGProvider{
		cacheDir: cacheDir,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *theSVGProvider) Name() string { return "thesvg" }

func pickVariant(variants []string, variant string) string {
	if variant == "color" {
		variant = "default"
	}
	if slices.Contains(variants, variant) {
		return variant
	}
	return "default"
}

func (p *theSVGProvider) Find(query, variant string) ([]byte, error) {
	p.once.Do(func() { p.icons, p.err = p.loadRegistry() })
	if p.err != nil {
		return nil, p.err
	}
	icon, ok := p.match(query)
	if !ok {
		return nil, errIconNotFound
	}
	// thesvg calls the brand-colour variant "default".
	want := pickVariant(icon.Variants, variant)
	return p.get(fmt.Sprintf("%s/icons/%s/%s.svg", theSVGBase, url.PathEscape(icon.Slug), want), theSVGMaxIcon)
}

// match only accepts exact matches on slug, title or alias (ignoring case and
// punctuation) so a vague query never pulls in an unrelated brand's logo.
// Slug matches win, so "Steam" picks "steam" over "steam-badge".
func (p *theSVGProvider) matchSlug(q string) (theSVGIcon, bool) {
	for _, i := range p.icons {
		if normalizeQuery(i.Slug) == q {
			return i, true
		}
	}
	return theSVGIcon{}, false
}

func aliasMatch(i theSVGIcon, q string) (theSVGIcon, bool) {
	for _, a := range i.Aliases {
		if normalizeQuery(a) == q {
			return i, true
		}
	}
	return theSVGIcon{}, false
}

func (p *theSVGProvider) matchTitleOrAlias(q string) (theSVGIcon, bool) {
	for _, i := range p.icons {
		if normalizeQuery(i.Title) == q {
			return i, true
		}
		if icon, ok := aliasMatch(i, q); ok {
			return icon, true
		}
	}
	return theSVGIcon{}, false
}

func (p *theSVGProvider) match(query string) (theSVGIcon, bool) {
	q := normalizeQuery(query)
	if q == "" {
		return theSVGIcon{}, false
	}
	if icon, ok := p.matchSlug(q); ok {
		return icon, true
	}
	return p.matchTitleOrAlias(q)
}

func registryFresh(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(st.ModTime()) <= theSVGRegistryTTL
}

func staleRegistryOK(path string, err error) error {
	if _, statErr := os.Stat(path); statErr != nil {
		return fmt.Errorf("download registry: %w", err)
	}
	return nil
}

func writeRegistry(dir, path string, raw []byte) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, raw)
}

func readRegistry(path string) ([]theSVGIcon, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reg struct {
		Icons []theSVGIcon `json:"icons"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return reg.Icons, nil
}

func (p *theSVGProvider) refreshRegistry(path string) error {
	if registryFresh(path) {
		return nil
	}
	raw, err := p.get(theSVGBase+"/api/registry.json", theSVGMaxRegistry)
	if err != nil {
		return staleRegistryOK(path, err)
	}
	return writeRegistry(p.cacheDir, path, raw)
}

// loadRegistry returns the manifest, refreshing the local copy when it is
// older than theSVGRegistryTTL. A stale copy is used if the refresh fails.
func (p *theSVGProvider) loadRegistry() ([]theSVGIcon, error) {
	path := filepath.Join(p.cacheDir, "thesvg-registry.json")
	if err := p.refreshRegistry(path); err != nil {
		return nil, err
	}
	return readRegistry(path)
}

func readSVGBody(u string, resp *http.Response, limit int64) ([]byte, error) {
	if resp.StatusCode == http.StatusNotFound {
		return nil, errIconNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func (p *theSVGProvider) get(u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return readSVGBody(u, resp, limit)
}
