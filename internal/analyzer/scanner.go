package analyzer

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gtkpad/devclean/internal/safety"
)

// LargeFileThreshold is 100 MB in bytes (104,857,600 bytes).
const LargeFileThreshold int64 = 100 * 1024 * 1024

// ScanUpdateMsg communicates incremental directory calculation progress.
type ScanUpdateMsg struct {
	Path      string
	SizeBytes int64
	ItemCount int
	Scanned   bool
	Err       error
}

// Scanner performs shallow immediate scans and concurrent recursive size analysis.
type Scanner struct {
	blacklist     *safety.Blacklist
	includeHidden bool
	minSize       int64
	maxDepth      int
	sem           chan struct{}
	mu            sync.RWMutex
	cache         map[string]*Node
}

// NewScanner constructs a new directory scanner.
func NewScanner(bl *safety.Blacklist, includeHidden bool, minSize int64, maxDepth int) *Scanner {
	w := runtime.NumCPU() * 2
	if w < 4 {
		w = 4
	}
	return &Scanner{
		blacklist:     bl,
		includeHidden: includeHidden,
		minSize:       minSize,
		maxDepth:      maxDepth,
		sem:           make(chan struct{}, w),
		cache:         make(map[string]*Node),
	}
}

// ScanImmediate performs instant shallow inspection of root directory entries.
func (s *Scanner) ScanImmediate(rootPath string) (*Node, error) {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)

	s.mu.RLock()
	if cached, ok := s.cache[abs]; ok {
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	rootFi, err := os.Lstat(abs)
	if err != nil {
		return nil, err
	}

	isProtected := safety.IsCoreBlacklisted(abs) || (s.blacklist != nil && s.blacklist.IsBlacklisted(abs))
	rootNode := &Node{
		Path:        abs,
		Name:        filepath.Base(abs),
		Type:        ItemDir,
		ModTime:     rootFi.ModTime(),
		IsProtected: isProtected,
	}

	if !rootFi.IsDir() {
		rootNode.Type = ItemFile
		rootNode.SizeBytes = rootFi.Size()
		rootNode.ItemCount = 1
		rootNode.Scanned = true
		if rootNode.SizeBytes >= LargeFileThreshold {
			rootNode.IsLargeFile = true
		}
		s.mu.Lock()
		s.cache[abs] = rootNode
		s.mu.Unlock()
		return rootNode, nil
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return rootNode, err
	}

	for _, entry := range entries {
		name := entry.Name()
		if !s.includeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		childPath := filepath.Clean(filepath.Join(abs, name))
		fi, err := os.Lstat(childPath)
		if err != nil {
			continue
		}

		isSymlink := fi.Mode()&fs.ModeSymlink != 0
		isDir := fi.IsDir() && !isSymlink
		itemType := ItemFile
		if isDir {
			itemType = ItemDir
		} else if isSymlink {
			itemType = ItemSymlink
		}

		childProtected := safety.IsCoreBlacklisted(childPath) || (s.blacklist != nil && s.blacklist.IsBlacklisted(childPath))
		childNode := &Node{
			Path:        childPath,
			Name:        name,
			Type:        itemType,
			ModTime:     fi.ModTime(),
			Parent:      rootNode,
			IsProtected: childProtected,
		}

		if !isDir {
			childNode.SizeBytes = fi.Size()
			childNode.ItemCount = 1
			childNode.Scanned = true
			if childNode.SizeBytes >= LargeFileThreshold {
				childNode.IsLargeFile = true
			}
		}

		rootNode.Children = append(rootNode.Children, childNode)
	}

	s.mu.Lock()
	s.cache[abs] = rootNode
	s.mu.Unlock()

	return rootNode, nil
}
