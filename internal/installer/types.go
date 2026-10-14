package installer

import (
	"time"
)

// InstallerType categorizes the type of installer detected.
type InstallerType string

const (
	TypeDirectApp    InstallerType = "app_bundle"
	TypeDiskImage    InstallerType = "disk_image"
	TypeInstallerPkg InstallerType = "package"
	TypeLinuxPkg     InstallerType = "linux_package"
	TypeWindowsPkg   InstallerType = "windows_installer"
	TypeArchivePkg   InstallerType = "archive_installer"
)

// Candidate represents an identified installer eligible for cleaning.
type Candidate struct {
	Path      string        `json:"path"`
	SizeBytes int64         `json:"size_bytes"`
	ModTime   time.Time     `json:"mod_time"`
	Type      InstallerType `json:"type"`
	Detail    string        `json:"detail"`
}

// Options configures the installer scanner.
type Options struct {
	Paths    []string
	MaxDepth int
	MinAge   time.Duration
	MinSize  int64
}
