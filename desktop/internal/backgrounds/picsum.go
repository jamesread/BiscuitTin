package backgrounds

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jamesread/biscuittin/internal/fsutil"
	"go.uber.org/zap"
)

const (
	picsumList  = "https://picsum.photos/v2/list?page=%d&limit=100"
	picsumImage = "https://picsum.photos/id/%s/%d/%d.jpg"
	// The catalogue is a little over 1000 photos, i.e. about 10 pages.
	picsumPages = 10
)

// picsumProvider serves photos from Lorem Picsum, which are Unsplash photos
// under the Unsplash licence (free to use, no attribution required; the
// photographer is credited anyway). Picsum has no subject tags, so photos
// cannot be limited to scenery. Requests use the viewport size, then
// standard sizes of the same orientation if that fails.
type picsumProvider struct {
	dir      string
	client   *http.Client
	imageFmt string // default picsumImage; tests replace this
	pageN    int    // 0 = random page; tests set this
}

type picsumPhoto struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (p *picsumProvider) Name() string { return "picsum" }
func (p *picsumProvider) Dir() string  { return p.dir }

func (p *picsumProvider) Next(size Size) (string, string, error) {
	photos, err := p.page(p.pageIndex())
	if err != nil {
		return "", "", err
	}
	candidates := picsumOriented(photos, size)
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("picsum: no photos on page")
	}

	ph := candidates[rand.IntN(len(candidates))]
	credit := ph.Author + "  /  Unsplash via Lorem Picsum"

	var lastErr error
	for _, dim := range requestSizes(size) {
		path := filepath.Join(p.dir, fmt.Sprintf("picsum-%s-%dx%d.jpg", ph.ID, dim.W, dim.H))
		url := fmt.Sprintf(p.imageURLFmt(), ph.ID, dim.W, dim.H)
		if err := download(p.client, url, path, credit); err != nil {
			lg().Warn("picsum size failed", zap.Int("w", dim.W), zap.Int("h", dim.H), zap.Error(err))
			lastErr = err
			continue
		}
		if _, err := decodeRGBA(path); err != nil {
			lg().Warn("picsum image unreadable", zap.String("path", path), zap.Error(err))
			_ = os.Remove(path)
			_ = os.Remove(path + ".credit")
			lastErr = err
			continue
		}
		return path, credit, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("picsum: no usable image")
	}
	return "", "", lastErr
}

func (p *picsumProvider) imageURLFmt() string {
	if p.imageFmt != "" {
		return p.imageFmt
	}
	return picsumImage
}

func (p *picsumProvider) pageIndex() int {
	if p.pageN > 0 {
		return p.pageN
	}
	return 1 + rand.IntN(picsumPages)
}

// picsumOriented prefers photos that match the viewport orientation so
// Picsum crops less. If the page has none, every photo is used.
func picsumOriented(photos []picsumPhoto, size Size) []picsumPhoto {
	size = normalizeSize(size)
	if size.W == size.H {
		return photos
	}
	wantPortrait := size.H > size.W
	var match []picsumPhoto
	for _, ph := range photos {
		if (ph.Height > ph.Width) == wantPortrait {
			match = append(match, ph)
		}
	}
	if len(match) > 0 {
		return match
	}
	return photos
}

// page returns one page of the photo list, cached on disk.
func (p *picsumProvider) page(n int) ([]picsumPhoto, error) {
	cache := filepath.Join(p.dir, "lists", fmt.Sprintf("page-%d.json", n))

	raw, err := os.ReadFile(cache)
	if err != nil {
		raw, err = httpGet(p.client, fmt.Sprintf(picsumList, n), 1<<20)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(cache), 0o755); err == nil {
			_ = fsutil.WriteFileAtomic(cache, raw)
		}
	}

	var photos []picsumPhoto
	if err := json.Unmarshal(raw, &photos); err != nil {
		_ = os.Remove(cache)
		return nil, err
	}
	return photos, nil
}
