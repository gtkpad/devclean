package installer

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

var partialExtensions = map[string]bool{
	".crdownload": true,
	".part":       true,
	".download":   true,
	".aria2":      true,
}

var directInstallers = map[string]InstallerType{
	".dmg":      TypeDiskImage,
	".iso":      TypeDiskImage,
	".pkg":      TypeInstallerPkg,
	".deb":      TypeLinuxPkg,
	".rpm":      TypeLinuxPkg,
	".appimage": TypeLinuxPkg,
	".exe":      TypeWindowsPkg,
	".msi":      TypeWindowsPkg,
}

// Detector identifies whether filesystem entries are installer files.
type Detector struct{}

// NewDetector creates a new Detector.
func NewDetector() *Detector {
	return &Detector{}
}

// Detect analyzes a path and its FileInfo to determine if it is an installer.
func (d *Detector) Detect(path string, info fs.FileInfo) (*Candidate, bool, error) {
	if info.IsDir() {
		return nil, false, nil
	}

	name := info.Name()
	lower := strings.ToLower(name)
	ext := strings.ToLower(filepath.Ext(lower))

	// D-03: Ignore partial / in-progress downloads
	if partialExtensions[ext] {
		return nil, false, nil
	}

	// D-04: Direct installer extensions
	if iType, ok := directInstallers[ext]; ok {
		return &Candidate{
			Path:      path,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
			Type:      iType,
			Detail:    formatDetail(iType, ext),
		}, true, nil
	}

	// D-01: Compressed archives with in-memory inspection
	if ext == ".zip" {
		matched, detail, err := InspectZip(path)
		if err != nil || !matched {
			return nil, false, err
		}
		return &Candidate{
			Path:      path,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
			Type:      TypeArchivePkg,
			Detail:    detail,
		}, true, nil
	}

	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		matched, detail, err := InspectTarGz(path)
		if err != nil || !matched {
			return nil, false, err
		}
		return &Candidate{
			Path:      path,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
			Type:      TypeArchivePkg,
			Detail:    detail,
		}, true, nil
	}

	return nil, false, nil
}

func formatDetail(iType InstallerType, ext string) string {
	switch iType {
	case TypeDiskImage:
		return fmt.Sprintf("imagem de disco (%s)", ext)
	case TypeInstallerPkg:
		return fmt.Sprintf("pacote de instalação (%s)", ext)
	case TypeLinuxPkg:
		return fmt.Sprintf("pacote linux (%s)", ext)
	case TypeWindowsPkg:
		return fmt.Sprintf("instalador windows (%s)", ext)
	case TypeDirectApp:
		return "aplicativo macOS (.app)"
	case TypeArchivePkg:
		return fmt.Sprintf("arquivo compactado (%s)", ext)
	default:
		return fmt.Sprintf("instalador (%s)", ext)
	}
}
