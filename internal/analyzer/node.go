package analyzer

import (
	"path/filepath"
	"time"
)

// ItemType classifies a filesystem node.
type ItemType int

const (
	ItemDir ItemType = iota
	ItemFile
	ItemSymlink
)

// Node represents a file, directory, or symlink in the analyzed directory tree.
type Node struct {
	Path        string
	Name        string
	Type        ItemType
	SizeBytes   int64
	ItemCount   int
	ModTime     time.Time
	Children    []*Node
	Parent      *Node
	IsProtected bool   // Matches safety.IsCoreBlacklisted or safety.Blacklist
	IsLargeFile bool   // Size >= LargeFileThreshold (100 MB)
	Scanned     bool   // Recursive scanning completed
	DeviceID    uint64 // Device/filesystem ID for mount boundary isolation
	HasDevID    bool   // Whether DeviceID was successfully resolved
}

// IsDir returns true if the node represents a directory.
func (n *Node) IsDir() bool {
	return n.Type == ItemDir
}

// IsSymlink returns true if the node represents a symbolic link.
func (n *Node) IsSymlink() bool {
	return n.Type == ItemSymlink
}

// Breadcrumbs returns formatted slice of parent path components for the header.
func (n *Node) Breadcrumbs() string {
	clean := filepath.Clean(n.Path)
	if clean == "/" {
		return "/"
	}
	return clean + "/"
}
