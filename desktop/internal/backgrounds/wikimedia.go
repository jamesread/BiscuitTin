package backgrounds

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jamesread/biscuittin/internal/fsutil"
)

const (
	wikimediaAPI   = "https://commons.wikimedia.org/w/api.php"
	wikimediaTries = 8
)

// Featured Commons categories that are places and scenery, not people or
// religious art. Picture of the Day is any topic, so it is not used.
var wikimediaSafeCategories = []string{
	"Category:Featured_pictures_of_landscapes",
	"Category:Featured_pictures_of_cityscapes",
	"Category:Featured_pictures_of_skylines",
	"Category:Featured_pictures_of_mountains",
	"Category:Featured_pictures_of_coasts",
	"Category:Featured_pictures_of_forests",
	"Category:Featured_pictures_of_bodies_of_water",
	"Category:Featured_pictures_of_islands",
	"Category:Featured_pictures_of_parks",
	"Category:Featured_pictures_of_volcanoes",
}

// Thumbnail URLs end ".../<width>px-<name>"; 1920 is one of Wikimedia's
// standard thumbnail widths.
var wikimediaThumbWidth = regexp.MustCompile(`/\d+px-([^/?]+)`)
var htmlTag = regexp.MustCompile(`<[^>]+>`)

// wikimediaProvider serves featured Wikimedia Commons photos of landscapes,
// cities and similar scenery. Every file is freely licensed (public domain
// or Creative Commons); the author and licence are shown as the credit.
type wikimediaProvider struct {
	dir    string
	client *http.Client
	api    string // default wikimediaAPI; tests replace this
	cats   []string
}

var errWikimediaSkip = errors.New("no usable picture in this page")

func (p *wikimediaProvider) Name() string { return "wikimedia" }
func (p *wikimediaProvider) Dir() string  { return p.dir }

func (p *wikimediaProvider) apiURL() string {
	if p.api != "" {
		return p.api
	}
	return wikimediaAPI
}

func (p *wikimediaProvider) categories() []string {
	if len(p.cats) > 0 {
		return p.cats
	}
	return wikimediaSafeCategories
}

// purgeLegacyPOTD drops Picture of the Day caches from before the scenic
// filter, including recent stills credited to Wikimedia, so old people and
// religious photos are not shown again.
func purgeLegacyPOTD(wikiDir, recents string) {
	feeds := filepath.Join(wikiDir, "feeds")
	if _, err := os.Stat(feeds); err != nil {
		return
	}
	for _, path := range cachedImages(wikiDir) {
		_ = os.Remove(path)
		_ = os.Remove(path + ".credit")
	}
	_ = os.RemoveAll(feeds)
	for _, path := range cachedImages(recents) {
		if strings.Contains(readCredit(path), "Wikimedia") {
			_ = os.Remove(path)
			_ = os.Remove(path + ".credit")
		}
	}
}

func (p *wikimediaProvider) Next(size Size) (string, string, error) {
	var lastErr error
	cats := p.categories()
	for i := 0; i < wikimediaTries; i++ {
		cat := cats[rand.IntN(len(cats))]
		path, credit, err := p.fromCategory(cat, size)
		if err == nil {
			return path, credit, nil
		}
		lastErr = err
		if !errors.Is(err, errWikimediaSkip) {
			break
		}
	}
	return "", "", lastErr
}

func (p *wikimediaProvider) fromCategory(cat string, size Size) (string, string, error) {
	pages, err := p.categoryPage(cat)
	if err != nil {
		return "", "", err
	}
	var usable []commonsPage
	for _, pg := range pages {
		if pg.usable(size) {
			usable = append(usable, pg)
		}
	}
	if len(usable) == 0 {
		return "", "", errWikimediaSkip
	}
	pg := usable[rand.IntN(len(usable))]
	return p.downloadPage(pg, size)
}

func (p *wikimediaProvider) downloadPage(pg commonsPage, size Size) (string, string, error) {
	info := pg.ImageInfo[0]
	credit := "Wikimedia Commons"
	if a := strings.TrimSpace(htmlTag.ReplaceAllString(info.meta("Artist"), "")); a != "" {
		credit = a + "  /  " + credit
	}
	if l := info.meta("LicenseShortName"); l != "" {
		credit += "  /  " + l
	}

	sum := sha1.Sum([]byte(pg.Title))
	id := hex.EncodeToString(sum[:8])
	var lastErr error
	for _, w := range wikimediaThumbWidths(size, info.Width) {
		url, ok := wikimediaURL(info.URL, info.ThumbURL, info.Width, w)
		if !ok {
			lastErr = errWikimediaSkip
			continue
		}
		path := filepath.Join(p.dir, fmt.Sprintf("%s-%d.jpg", id, w))
		if err := download(p.client, url, path, credit); err != nil {
			lastErr = err
			continue
		}
		if _, err := decodeRGBA(path); err != nil {
			_ = os.Remove(path)
			_ = os.Remove(path + ".credit")
			lastErr = err
			continue
		}
		return path, credit, nil
	}
	if lastErr == nil {
		lastErr = errWikimediaSkip
	}
	return "", "", lastErr
}

type commonsQuery struct {
	Query struct {
		Pages map[string]commonsPage `json:"pages"`
	} `json:"query"`
}

type commonsPage struct {
	Title     string        `json:"title"`
	ImageInfo []commonsInfo `json:"imageinfo"`
}

type commonsInfo struct {
	Width    int               `json:"width"`
	Height   int               `json:"height"`
	URL      string            `json:"url"`
	ThumbURL string            `json:"thumburl"`
	ExtMeta  map[string]extVal `json:"extmetadata"`
}

type extVal struct {
	Value json.RawMessage `json:"value"`
}

func (e extVal) String() string {
	var s string
	if json.Unmarshal(e.Value, &s) == nil {
		return s
	}
	return strings.Trim(string(e.Value), `"`)
}

func (info commonsInfo) meta(key string) string {
	if info.ExtMeta == nil {
		return ""
	}
	return info.ExtMeta[key].String()
}

func (pg commonsPage) usable(size Size) bool {
	if len(pg.ImageInfo) == 0 {
		return false
	}
	lower := strings.ToLower(pg.Title)
	if !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
		return false
	}
	info := pg.ImageInfo[0]
	if !wikimediaOrientationOK(info.Width, info.Height, size) {
		return false
	}
	return sceneSafe(pg.Title, info.meta("ImageDescription"), info.meta("Categories"), info.meta("ObjectName"))
}

func (p *wikimediaProvider) categoryPage(cat string) ([]commonsPage, error) {
	prefix := randomSortPrefix()
	cache := filepath.Join(p.dir, "lists", safeCacheName(cat)+"-"+prefix+".json")

	raw, err := os.ReadFile(cache)
	if err != nil {
		q := url.Values{
			"action":                {"query"},
			"format":                {"json"},
			"generator":             {"categorymembers"},
			"gcmtitle":              {cat},
			"gcmtype":               {"file"},
			"gcmlimit":              {"20"},
			"gcmstartsortkeyprefix": {prefix},
			"prop":                  {"imageinfo"},
			"iiprop":                {"url|size|extmetadata"},
			"iiurlwidth":            {fmt.Sprintf("%d", bgWidth)},
		}
		raw, err = httpGet(p.client, p.apiURL()+"?"+q.Encode(), 4<<20)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(cache), 0o755); err == nil {
			_ = fsutil.WriteFileAtomic(cache, raw)
		}
	}

	doc := &commonsQuery{}
	if err := json.Unmarshal(raw, doc); err != nil || len(doc.Query.Pages) == 0 {
		_ = os.Remove(cache)
		return nil, errWikimediaSkip
	}
	pages := make([]commonsPage, 0, len(doc.Query.Pages))
	for _, pg := range doc.Query.Pages {
		pages = append(pages, pg)
	}
	return pages, nil
}

func randomSortPrefix() string {
	if rand.IntN(6) == 0 {
		return ""
	}
	return string(rune('A' + rand.IntN(26)))
}

func safeCacheName(cat string) string {
	s := strings.ToLower(strings.TrimPrefix(cat, "Category:"))
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "cat"
	}
	return b.String()
}

// wikimediaOrientationOK is true for images that match the viewport, or for
// a square viewport (any image).
func wikimediaOrientationOK(imgW, imgH int, size Size) bool {
	size = normalizeSize(size)
	if size.W == size.H {
		return true
	}
	return (imgH > imgW) == (size.H > size.W)
}

// wikimediaThumbWidths is the viewport width, then 1920 if that is different.
func wikimediaThumbWidths(size Size, originalW int) []int {
	size = normalizeSize(size)
	seen := map[int]bool{}
	var out []int
	for _, w := range []int{size.W, bgWidth} {
		if w <= 0 || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	if originalW > 0 && !seen[originalW] {
		out = append(out, originalW)
	}
	return out
}

func wikimediaURL(original, thumb string, originalW, wantW int) (string, bool) {
	if originalW > 0 && originalW <= wantW {
		return original, original != ""
	}
	if thumb == "" || !wikimediaThumbWidth.MatchString(thumb) {
		return original, original != ""
	}
	return wikimediaThumbWidth.ReplaceAllString(thumb, fmt.Sprintf("/%dpx-$1", wantW)), true
}
