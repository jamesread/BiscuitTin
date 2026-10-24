package icons

import (
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
	cacheDir string
	client   *http.Client

	once  sync.Once
	icons []theSVGIcon
	err   error
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
	want := variant
	if want == "color" {
		want = "default"
	}
	if !slices.Contains(icon.Variants, want) {
		want = "default"
	}

	return p.get(fmt.Sprintf("%s/icons/%s/%s.svg", theSVGBase, url.PathEscape(icon.Slug), want), theSVGMaxIcon)
}

// match only accepts exact matches on slug, title or alias (ignoring case and
// punctuation) so a vague query never pulls in an unrelated brand's logo.
// Slug matches win, so "Steam" picks "steam" over "steam-badge".
func (p *theSVGProvider) match(query string) (theSVGIcon, bool) {
	q := normalizeQuery(query)
	if q == "" {
		return theSVGIcon{}, false
	}
	for _, i := range p.icons {
		if normalizeQuery(i.Slug) == q {
			return i, true
		}
	}
	for _, i := range p.icons {
		if normalizeQuery(i.Title) == q {
			return i, true
		}
		for _, a := range i.Aliases {
			if normalizeQuery(a) == q {
				return i, true
			}
		}
	}
	return theSVGIcon{}, false
}

// loadRegistry returns the manifest, refreshing the local copy when it is
// older than theSVGRegistryTTL. A stale copy is used if the refresh fails.
func (p *theSVGProvider) loadRegistry() ([]theSVGIcon, error) {
	path := filepath.Join(p.cacheDir, "thesvg-registry.json")

	st, statErr := os.Stat(path)
	if statErr != nil || time.Since(st.ModTime()) > theSVGRegistryTTL {
		raw, err := p.get(theSVGBase+"/api/registry.json", theSVGMaxRegistry)
		if err == nil {
			if err := os.MkdirAll(p.cacheDir, 0o755); err != nil {
				return nil, err
			}
			if err := fsutil.WriteFileAtomic(path, raw); err != nil {
				return nil, err
			}
		} else if statErr != nil {
			return nil, fmt.Errorf("download registry: %w", err)
		}
	}

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

func (p *theSVGProvider) get(u string, limit int64) ([]byte, error) {
	resp, err := p.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errIconNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}
