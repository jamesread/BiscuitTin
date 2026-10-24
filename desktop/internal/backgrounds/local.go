package backgrounds

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// localProvider shows images from a directory of your own (JPEG or PNG), in
// file name order, with no downloads. A "<image>.credit" file next to an
// image gives its attribution line.
type localProvider struct {
	dir  string
	next int
}

func (p *localProvider) Name() string { return "local" }
func (p *localProvider) Dir() string  { return p.dir }

func (p *localProvider) Next(Size) (string, string, error) {
	files := localImages(p.dir)
	if len(files) == 0 {
		return "", "", fmt.Errorf("local: no .jpg or .png images in %s", p.dir)
	}
	path := files[p.next%len(files)]
	p.next++
	return path, readCredit(path), nil
}

func localImages(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*"))
	var files []string
	for _, m := range matches {
		switch strings.ToLower(filepath.Ext(m)) {
		case ".jpg", ".jpeg", ".png":
			files = append(files, m)
		}
	}
	sort.Strings(files)
	return files
}
