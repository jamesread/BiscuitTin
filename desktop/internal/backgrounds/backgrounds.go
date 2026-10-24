// Package backgrounds fetches royalty-free background images (Wikimedia
// Commons, Lorem Picsum or a local directory) on a background goroutine.
package backgrounds

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/config"
	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/fsutil"
	"go.uber.org/zap"
)

const (
	bgUserAgent     = "biscuittin/0.1 (https://github.com/jamesread/biscuittin)"
	bgMaxDownload   = 20 << 20
	bgKeepPerSource = 150
	recentKeep      = 5
	bgWidth         = 1920
	bgHeight        = 1080
)

// bgProvider supplies royalty-free background images. Next downloads (or
// reuses) one image in the provider's cache directory and returns its path
// and an attribution line to show on screen. size is the current viewport;
// providers that accept a resolution should use it, then fall back to a
// standard aspect ratio if that fails.
type bgProvider interface {
	Name() string
	Dir() string
	Next(size Size) (path, credit string, err error)
}

func newBgProvider(name string, cfg *config.Config) (bgProvider, error) {
	dir := filepath.Join(cfg.CacheDir, "backgrounds", name)
	switch name {
	case "local":
		if cfg.Backgrounds.Dir == "" {
			return nil, fmt.Errorf("the local provider needs backgrounds.dir")
		}
		return &localProvider{dir: cfg.Backgrounds.Dir}, nil
	case "wikimedia":
		return &wikimediaProvider{dir: dir, client: newBgClient()}, nil
	case "picsum":
		return &picsumProvider{dir: dir, client: newBgClient()}, nil
	}
	return nil, fmt.Errorf("unknown background provider %q", name)
}

func newBgClient() *http.Client {
	return &http.Client{Timeout: 45 * time.Second}
}

// Image is a decoded background, ready to upload as a texture.
type Image struct {
	Err    error
	Pix    *image.RGBA
	Grid   *contrast.LumGrid
	Credit string
}

// Fetcher runs providers on a background goroutine so downloads and JPEG
// decoding never stall rendering. Each request yields one result on out.
type Fetcher struct {
	requests  chan Size
	Out       chan Image
	recentDir string
	providers []bgProvider
	next      int
}

func NewFetcher(cfg *config.Config) *Fetcher {
	f := &Fetcher{
		requests:  make(chan Size, 1),
		Out:       make(chan Image, 1),
		recentDir: recentDir(cfg.CacheDir),
	}
	pruneCache(f.recentDir, recentKeep)
	purgeLegacyPOTD(filepath.Join(cfg.CacheDir, "backgrounds", "wikimedia"), f.recentDir)
	for _, name := range cfg.Backgrounds.Providers {
		p, err := newBgProvider(name, cfg)
		if err != nil {
			lg().Warn("background provider", zap.Error(err))
			continue
		}
		f.providers = append(f.providers, p)
	}
	go f.run()
	return f
}

func (f *Fetcher) Request(w, h int) {
	select {
	case f.requests <- Size{W: w, H: h}:
	default:
	}
}

func (f *Fetcher) run() {
	for size := range f.requests {
		f.Out <- f.fetch(size)
	}
}

// fetch tries each provider in turn, rotating the starting provider so
// consecutive images alternate between sources.
func (f *Fetcher) decodeFetched(i int, path, credit string) (Image, error, bool) {
	pix, err := decodeRGBA(path)
	if err != nil {
		lg().Warn("decode failed, removing", zap.String("path", path), zap.Error(err))
		_ = os.Remove(path)
		return Image{}, err, false
	}
	f.next = (f.next + i + 1) % len(f.providers)
	if err := saveRecent(f.recentDir, path, credit); err != nil {
		lg().Warn("could not cache recent background", zap.Error(err))
	}
	return Image{Pix: pix, Grid: contrast.BuildLumGrid(pix), Credit: credit}, nil, true
}

func (f *Fetcher) tryProvider(i int, size Size) (Image, error, bool) {
	p := f.providers[(f.next+i)%len(f.providers)]
	path, credit, err := p.Next(size)
	if err != nil {
		lg().Warn("fetch failed, trying the cache", zap.String("provider", p.Name()), zap.Error(err))
		path, credit, err = randomCached(p.Dir(), p.Name())
		if err != nil {
			return Image{}, err, false
		}
	}
	return f.decodeFetched(i, path, credit)
}

func (f *Fetcher) fetchFromProviders(size Size) Image {
	var lastErr error
	for i := range f.providers {
		img, err, ok := f.tryProvider(i, size)
		if ok {
			return img
		}
		lastErr = err
	}
	return Image{Err: lastErr}
}

func (f *Fetcher) fetch(size Size) Image {
	if len(f.providers) == 0 {
		return Image{Err: fmt.Errorf("no background providers configured")}
	}
	return f.fetchFromProviders(size)
}

func decodeRGBA(path string) (*image.RGBA, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fh.Close() }()

	src, _, err := image.Decode(fh)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst, nil
}

// randomCached picks a previously downloaded image when a provider cannot
// reach the network, so backgrounds keep rotating offline.
func randomCached(dir, provider string) (string, string, error) {
	files := cachedImages(dir)
	if len(files) == 0 {
		return "", "", fmt.Errorf("%s: no cached images", provider)
	}
	path := files[rand.IntN(len(files))]
	return path, readCredit(path), nil
}

func cachedImages(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*"))
	var files []string
	for _, m := range matches {
		switch strings.ToLower(filepath.Ext(m)) {
		case ".jpg", ".jpeg", ".png":
			files = append(files, m)
		}
	}
	return files
}

func readCredit(imagePath string) string {
	raw, _ := os.ReadFile(imagePath + ".credit")
	return strings.TrimSpace(string(raw))
}

// download saves url to path (with its credit alongside) unless already
// cached, then prunes the directory to the newest bgKeepPerSource images.
func writeDownload(client *http.Client, url, path, credit string) error {
	raw, err := httpGet(client, url, bgMaxDownload)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path+".credit", []byte(credit+"\n")); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, raw); err != nil {
		return err
	}
	pruneCache(filepath.Dir(path), bgKeepPerSource)
	return nil
}

func download(client *http.Client, url, path, credit string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return writeDownload(client, url, path, credit)
}

func httpGet(client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", bgUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func pruneCache(dir string, keep int) {
	files := cachedImages(dir)
	if len(files) <= keep {
		return
	}
	sortByModTimeNewest(files)
	for _, p := range files[keep:] {
		_ = os.Remove(p)
		_ = os.Remove(p + ".credit")
	}
}

func sortByModTimeNewest(files []string) {
	mtime := func(p string) time.Time {
		st, err := os.Stat(p)
		if err != nil {
			return time.Time{}
		}
		return st.ModTime()
	}
	sort.Slice(files, func(i, j int) bool { return mtime(files[i]).After(mtime(files[j])) })
}

// lg is this package's logger. main installs the global logger.
func lg() *zap.Logger { return zap.L().Named("backgrounds") }
