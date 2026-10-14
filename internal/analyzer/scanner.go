package analyzer

import (
	"context"
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

	rootDev, hasDev := getDeviceID(rootFi)
	isProtected := safety.IsCoreBlacklisted(abs) || (s.blacklist != nil && s.blacklist.IsBlacklisted(abs))
	rootNode := &Node{
		Path:        abs,
		Name:        filepath.Base(abs),
		Type:        ItemDir,
		DeviceID:    rootDev,
		HasDevID:    hasDev,
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

		childDev, childHasDev := getDeviceID(fi)
		childProtected := rootNode.IsProtected || safety.IsCoreBlacklisted(childPath) || (s.blacklist != nil && s.blacklist.IsBlacklisted(childPath))
		childNode := &Node{
			Path:        childPath,
			Name:        name,
			Type:        itemType,
			DeviceID:    childDev,
			HasDevID:    childHasDev,
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

// CalculateDirSize recursively calculates size and item count, skipping symlink traversals and foreign mount points.
func (s *Scanner) CalculateDirSize(ctx context.Context, dirNode *Node, updates chan<- ScanUpdateMsg) (int64, int, error) {
	if dirNode == nil {
		return 0, 0, nil
	}

	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	}

	var totalBytes int64
	var itemCount int

	err := filepath.WalkDir(dirNode.Path, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if err != nil {
			if path == dirNode.Path {
				return err
			}
			return nil
		}

		if path == dirNode.Path {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// D-02: Prevent following symlinks (T-04-01)
		if info.Mode()&fs.ModeSymlink != 0 {
			totalBytes += info.Size()
			itemCount++
			return nil
		}

		// D-02: Prevent crossing mount points (T-04-02)
		if dirNode.HasDevID && d.IsDir() {
			devID, hasDev := getDeviceID(info)
			if hasDev && devID != dirNode.DeviceID {
				return filepath.SkipDir
			}
		}

		if !d.IsDir() {
			sz := info.Size()
			totalBytes += sz
			itemCount++
		} else {
			itemCount++
		}
		return nil
	})

	if err == nil && updates == nil {
		dirNode.SizeBytes = totalBytes
		dirNode.ItemCount = itemCount
		dirNode.Scanned = true
	}

	if updates != nil {
		select {
		case updates <- ScanUpdateMsg{
			Path:      dirNode.Path,
			SizeBytes: totalBytes,
			ItemCount: itemCount,
			Scanned:   err == nil,
			Err:       err,
		}:
		case <-ctx.Done():
		}
	}

	return totalBytes, itemCount, err
}
