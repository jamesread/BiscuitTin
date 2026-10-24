package backgrounds

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jamesread/biscuittin/internal/contrast"
	"github.com/jamesread/biscuittin/internal/fsutil"
	"go.uber.org/zap"
)

func recentDir(cacheDir string) string {
	return filepath.Join(cacheDir, "backgrounds", "recent")
}

// LoadRecent returns the most recently shown background, after pruning the
// recent cache to recentKeep. ok is false when there is nothing usable.
func LoadRecent(cacheDir string) (Image, bool) {
	dir := recentDir(cacheDir)
	pruneCache(dir, recentKeep)
	files := cachedImages(dir)
	if len(files) == 0 {
		return Image{}, false
	}
	sortByModTimeNewest(files)
	for _, path := range files {
		pix, err := decodeRGBA(path)
		if err != nil {
			lg().Warn("recent background unreadable, removing", zap.String("path", path), zap.Error(err))
			_ = os.Remove(path)
			_ = os.Remove(path + ".credit")
			continue
		}
		return Image{Pix: pix, Grid: contrast.BuildLumGrid(pix), Credit: readCredit(path)}, true
	}
	return Image{}, false
}

func saveRecent(dir, src, credit string) error {
	if dir == "" || src == "" {
		return nil
	}
	if sameDir(src, dir) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".jpg", ".jpeg", ".png":
	default:
		ext = ".jpg"
	}
	dest := filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000")+ext)
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(dest+".credit", []byte(credit+"\n")); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(dest, raw); err != nil {
		return err
	}
	pruneCache(dir, recentKeep)
	return nil
}

func sameDir(path, dir string) bool {
	return filepath.Clean(filepath.Dir(path)) == filepath.Clean(dir)
}
