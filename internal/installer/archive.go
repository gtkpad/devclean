package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var codeSourceMarkers = map[string]bool{
	"package.json":    true,
	"go.mod":          true,
	"cargo.toml":      true,
	"pom.xml":         true,
	"build.gradle":    true,
	"build.gradle.kts": true,
	"cmake_lists.txt": true,
	"cmakelists.txt":  true,
}

var executableExtensions = map[string]bool{
	".exe":      true,
	".msi":      true,
	".pkg":      true,
	".deb":      true,
	".rpm":      true,
	".appimage": true,
}

var (
	magicELF       = []byte{0x7f, 'E', 'L', 'F'}
	magicMachO32   = []byte{0xfe, 0xed, 0xfa, 0xce}
	magicMachO64   = []byte{0xfe, 0xed, 0xfa, 0xcf}
	magicMachOCig32 = []byte{0xce, 0xfa, 0xed, 0xfe}
	magicMachOCig64 = []byte{0xcf, 0xfa, 0xed, 0xfe}
	magicPE        = []byte{'M', 'Z'}
)

// isExecutableMagic inspects the first few bytes (up to 4) of a file to check for executable magic numbers.
func isExecutableMagic(buf []byte) bool {
	if len(buf) >= 4 {
		if bytes.Equal(buf[:4], magicELF) ||
			bytes.Equal(buf[:4], magicMachO32) ||
			bytes.Equal(buf[:4], magicMachO64) ||
			bytes.Equal(buf[:4], magicMachOCig32) ||
			bytes.Equal(buf[:4], magicMachOCig64) {
			return true
		}
	}
	if len(buf) >= 2 && bytes.Equal(buf[:2], magicPE) {
		return true
	}
	return false
}

// InspectZip inspects the Central Directory of a zip archive looking for installers.
// It performs zero disk extraction, operating purely in memory on header metadata.
func InspectZip(path string) (bool, string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false, "", err
	}
	defer zr.Close()

	hasSource := false
	var matched bool
	var matchedDetail string

	maxEntries := len(zr.File)
	if maxEntries > 200 {
		maxEntries = 200
	}

	for i := 0; i < maxEntries; i++ {
		f := zr.File[i]
		base := strings.ToLower(filepath.Base(f.Name))
		ext := strings.ToLower(filepath.Ext(base))

		if codeSourceMarkers[base] {
			hasSource = true
		}

		if !matched {
			if strings.Contains(f.Name, ".app/Contents/MacOS/") {
				matched = true
				matchedDetail = "contém aplicativo macOS (.app)"
			} else if executableExtensions[ext] {
				matched = true
				matchedDetail = fmt.Sprintf("contém instalador %s", filepath.Base(f.Name))
			} else if ext == "" && !f.FileInfo().IsDir() && f.UncompressedSize64 > 1024 {
				if matchExecutableZipHeader(f) {
					matched = true
					matchedDetail = fmt.Sprintf("contém binário executável %s", filepath.Base(f.Name))
				}
			}
		}
	}

	if hasSource {
		return false, "arquivo de código-fonte descartado", nil
	}

	if matched {
		return true, matchedDetail, nil
	}

	return false, "", nil
}

func matchExecutableZipHeader(f *zip.File) bool {
	rc, err := f.Open()
	if err != nil {
		return false
	}
	defer rc.Close()

	buf := make([]byte, 4)
	n, err := io.ReadFull(rc, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return false
	}
	if n < 2 {
		return false
	}
	return isExecutableMagic(buf[:n])
}

// InspectTarGz inspects the first headers of a .tar.gz / .tgz archive looking for installers.
// It checks up to 100 headers in memory without extracting anything to disk.
func InspectTarGz(path string) (bool, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return false, "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	hasSource := false
	var matched bool
	var matchedDetail string
	checked := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF || checked >= 100 {
			break
		}
		if err != nil {
			return false, "", err
		}
		checked++

		base := strings.ToLower(filepath.Base(hdr.Name))
		ext := strings.ToLower(filepath.Ext(base))

		if codeSourceMarkers[base] {
			hasSource = true
		}

		if !matched {
			if strings.Contains(hdr.Name, ".app/Contents/MacOS/") {
				matched = true
				matchedDetail = "contém aplicativo macOS (.app)"
			} else if executableExtensions[ext] {
				matched = true
				matchedDetail = fmt.Sprintf("contém instalador %s", filepath.Base(hdr.Name))
			} else if ext == "" && hdr.Typeflag != tar.TypeDir && hdr.Size > 1024 {
				buf := make([]byte, 4)
				n, _ := io.ReadFull(tr, buf)
				if n >= 2 && isExecutableMagic(buf[:n]) {
					matched = true
					matchedDetail = fmt.Sprintf("contém binário executável %s", filepath.Base(hdr.Name))
				}
			}
		}
	}

	if hasSource {
		return false, "arquivo de código-fonte descartado", nil
	}

	if matched {
		return true, matchedDetail, nil
	}

	return false, "", nil
}
