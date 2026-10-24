// Package fsutil has small file system helpers.
package fsutil

import (
	"os"
)

// WriteFileAtomic writes data to a temporary file and renames it over path,
// so readers never see a partly written file.
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
