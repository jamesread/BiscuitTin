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
			removeImage(path)
			continue
		}
		return Image{Pix: pix, Grid: contrast.BuildLumGrid(pix), Credit: readCredit(path), Title: readTitle(path)}, true
	}
	return Image{}, false
}

func shouldSaveRecent(dir, src string) bool {
	if dir == "" {
		return false
	}
	if src == "" {
		return false
	}
	return !sameDir(src, dir)
}

func recentExt(src string) string {
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".jpg", ".jpeg", ".png":
		return ext
	}
	return ".jpg"
}

func writeRecentFiles(dest, src, credit, title, dir string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeSidecars(dest, credit, title); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(dest, raw); err != nil {
		return err
	}
	pruneCache(dir, recentKeep)
	return nil
}

func saveRecent(dir, src, credit, title string) error {
	if !shouldSaveRecent(dir, src) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	dest := filepath.Join(dir, time.Now().UTC().Format("20060102T150405.000000000")+recentExt(src))
	return writeRecentFiles(dest, src, credit, title, dir)
}

func sameDir(path, dir string) bool {
	return filepath.Clean(filepath.Dir(path)) == filepath.Clean(dir)
}
