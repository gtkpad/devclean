package fs

import "os"

// statPath performs an os.Stat. Separated for testability.
func statPath(path string) error {
	_, err := os.Stat(path)
	return err
}
