// Package icons finds, renders and caches tile icons from SVG icon providers
// (thesvg.org, Iconify).
package icons

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/fsutil"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"go.uber.org/zap"
)

const (
	// iconRenderVersion is part of cached icon file names; bump it when
	// rendering changes so stale PNGs are redrawn.
	iconRenderVersion = 2
	iconRasterSize    = 512
	// Failed lookups are not retried until this has passed, so an offline or
	// unmatched entry does not hit the network on every start.
	iconMissTTL = 24 * time.Hour
)

var errIconNotFound = errors.New("icon not found")

// iconProvider finds an SVG for a search query. Providers are tried in the
// order listed in the config's iconProviders.
type iconProvider interface {
	Name() string
	// Find returns the SVG document for query, or errIconNotFound.
	// variant is "mono" or "color"; providers fall back to whatever they have.
	Find(query, variant string) ([]byte, error)
}

func newIconProvider(name, cacheDir string) (iconProvider, error) {
	switch name {
	case "thesvg":
		return newTheSVGProvider(filepath.Join(cacheDir, "providers")), nil
	case "iconify":
		return newIconifyProvider(), nil
	}
	return nil, fmt.Errorf("unknown icon provider %q", name)
}

type Result struct {
	Key  string // tile key, e.g. "entry:0" or "ot:<bindingId>"
	Path string
}

// Resolve looks up icons for entries without an explicit icon file and
// sends each rendered PNG path to results, keyed by key(entry index), as it
// becomes available. It runs in the background so startup never waits on the
// network; textures are loaded by the main (GL) thread when the results
// arrive.
func iconProviders(cfg *config.Config) []iconProvider {
	var providers []iconProvider
	for _, name := range cfg.IconProviders {
		p, err := newIconProvider(name, cfg.CacheDir)
		if err != nil {
			lg().Warn("icon provider", zap.Error(err))
			continue
		}
		providers = append(providers, p)
	}
	return providers
}

func iconQuery(e config.Entry) (query, style string) {
	query = e.IconQuery
	if query == "" {
		query = e.Title
	}
	style = e.IconStyle
	if style == "" {
		style = "mono"
	}
	return query, style
}

func tryCachedIcon(p iconProvider, cacheDir, query, style, key string, results chan<- Result) bool {
	path, err := cachedIcon(p, cacheDir, query, style)
	if err == nil {
		results <- Result{Key: key, Path: path}
		return true
	}
	if !errors.Is(err, errIconNotFound) {
		lg().Warn("icon lookup failed", zap.String("provider", p.Name()), zap.String("query", query), zap.Error(err))
	}
	return false
}

func resolveEntry(providers []iconProvider, cacheDir string, i int, e config.Entry, key func(int) string, results chan<- Result) {
	if e.Icon != "" {
		return
	}
	query, style := iconQuery(e)
	for _, p := range providers {
		if tryCachedIcon(p, cacheDir, query, style, key(i), results) {
			return
		}
	}
}

func Resolve(cfg *config.Config, key func(entry int) string, results chan<- Result) {
	providers := iconProviders(cfg)
	cacheDir := filepath.Join(cfg.CacheDir, "icons")
	for i, e := range cfg.Entries {
		resolveEntry(providers, cacheDir, i, e, key, results)
	}
}

// ResolveIconify renders an Iconify icon ("prefix:name", as used by
// OliveTin) for a tile, sending the result when ready.
func ResolveIconify(cfg *config.Config, key, name string, results chan<- Result) {
	path, err := cachedIcon(newIconifyProvider(), filepath.Join(cfg.CacheDir, "icons"), name, "mono")
	if err != nil {
		if !errors.Is(err, errIconNotFound) {
			lg().Warn("icon lookup failed", zap.String("provider", "iconify"), zap.String("query", name), zap.Error(err))
		}
		return
	}
	results <- Result{Key: key, Path: path}
}

// cachedIcon returns a rendered PNG for query from provider p, rendering and
// caching it on first use.
type iconCache struct {
	dir  string
	png  string
	miss string
}

func iconCachePaths(p iconProvider, cacheDir, query, style string) iconCache {
	dir := filepath.Join(cacheDir, p.Name())
	base := fmt.Sprintf("%s-%s-r%d", normalizeQuery(query), style, iconRenderVersion)
	return iconCache{dir: dir, png: filepath.Join(dir, base+".png"), miss: filepath.Join(dir, base+".miss")}
}

func freshMiss(missPath string) bool {
	st, err := os.Stat(missPath)
	if err != nil {
		return false
	}
	return time.Since(st.ModTime()) < iconMissTTL
}

func iconCacheHit(paths iconCache) (bool, error) {
	if _, err := os.Stat(paths.png); err == nil {
		return true, nil
	}
	if freshMiss(paths.miss) {
		return true, errIconNotFound
	}
	return false, nil
}

func fetchIconSVG(p iconProvider, paths iconCache, query, style string) ([]byte, error) {
	svg, err := p.Find(query, style)
	if err == nil {
		return svg, nil
	}
	if errors.Is(err, errIconNotFound) {
		_ = os.MkdirAll(paths.dir, 0o750)
		_ = os.WriteFile(paths.miss, nil, 0o600)
		lg().Info("no icon found", zap.String("provider", p.Name()), zap.String("query", query))
	}
	return nil, err
}

func writePNGFile(pngPath, dir string, img *image.RGBA) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(pngPath, buf.Bytes())
}

func writeCachedPNG(p iconProvider, paths iconCache, query, style string, svg []byte) (string, error) {
	img, err := RasterizeSVG(svg, iconRasterSize)
	if err != nil {
		return "", fmt.Errorf("render: %w", err)
	}
	if style == "mono" {
		TintWhite(img)
	}
	if err := writePNGFile(paths.png, paths.dir, img); err != nil {
		return "", err
	}
	_ = os.Remove(paths.miss)
	lg().Info("icon cached", zap.String("provider", p.Name()), zap.String("query", query), zap.String("path", paths.png))
	return paths.png, nil
}

func cachedIcon(p iconProvider, cacheDir, query, style string) (string, error) {
	paths := iconCachePaths(p, cacheDir, query, style)
	if cached, err := iconCacheHit(paths); cached {
		return paths.png, err
	}
	svg, err := fetchIconSVG(p, paths, query, style)
	if err != nil {
		return "", err
	}
	return writeCachedPNG(p, paths, query, style, svg)
}

// RasterizeSVG renders an SVG into a size x size image, preserving the aspect
// ratio of its viewBox and centring it.
func svgViewSize(vw, vh float64) (float64, float64) {
	if vw > 0 && vh > 0 {
		return vw, vh
	}
	return 1, 1
}

func RasterizeSVG(svg []byte, size int) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(stripRootSize(svg)), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	vw, vh := svgViewSize(icon.ViewBox.W, icon.ViewBox.H)
	k := float64(size) / math.Max(vw, vh)
	w, h := vw*k, vh*k
	icon.SetTarget((float64(size)-w)/2, (float64(size)-h)/2, w, h)
	// oksvg applies the transform to path geometry but not to stroke widths,
	// which it treats as pixels; scale them so outline icons aren't hairlines.
	for i := range icon.SVGPaths {
		icon.SVGPaths[i].LineWidth *= k
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
	return img, nil
}

var (
	svgRootTag  = regexp.MustCompile(`(?s)<svg\b[^>]*>`)
	svgSizeAttr = regexp.MustCompile(`\s(width|height)="[^"]*"`)
)

// stripRootSize removes width/height from the root <svg> when it has a
// viewBox. Sizing comes from the viewBox anyway, and oksvg mis-scales units
// such as Iconify's width="1em", drawing icons as a solid square.
func stripRootSize(svg []byte) []byte {
	loc := svgRootTag.FindIndex(svg)
	if loc == nil {
		return svg
	}
	tag := svg[loc[0]:loc[1]]
	if !bytes.Contains(tag, []byte("viewBox")) {
		return svg
	}
	out := append([]byte{}, svg[:loc[0]]...)
	out = append(out, svgSizeAttr.ReplaceAll(tag, nil)...)
	return append(out, svg[loc[1]:]...)
}

// TintWhite makes every pixel white while keeping its coverage, so a mono
// glyph reads clearly on a coloured tile.
func TintWhite(img *image.RGBA) {
	for i := 0; i < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = a, a, a // premultiplied
	}
}

func keepQueryRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	return r >= '0' && r <= '9'
}

func normalizeQuery(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if keepQueryRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("icons") }
