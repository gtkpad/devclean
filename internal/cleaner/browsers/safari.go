package browsers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// scanSafari scans strictly ~/Library/Caches/com.apple.Safari on Darwin.
func scanSafari(
	ctx context.Context,
	homeDir, goos string,
	bDef BrowserDef,
	isProcessBlocked bool,
	cutoff time.Time,
	maxAgeDays int,
) ([]domain.Target, error) {
	if goos != "darwin" {
		return nil, nil
	}
	if bDef.CacheDir == nil {
		return nil, nil
	}

	cacheRoot := bDef.CacheDir(homeDir, "darwin")
	if cacheRoot == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		// Graceful degradation on TCC / Permission Denied (D-14)
		if os.IsPermission(err) || isSyscallPermission(err) {
			vPath := "browser://safari/caches"
			trg := domain.Target{
				ID:            hashBrowserPath(vPath),
				Name:          "Safari Caches",
				Path:          vPath,
				ReviewOnly:    true,
				ReviewReason:  domain.ReviewReasonSystemProtected,
				EligibleBytes: 0,
				TTLEligible:   false,
				Risk:          domain.RiskLow,
				ProviderMetrics: &domain.ProviderMetrics{
					Partial:    true,
					StatusNote: "permission denied (TCC / Full Disk Access required)",
				},
				Category: "browser-cache",
			}
			return []domain.Target{trg}, nil
		}
		return nil, err
	}

	var totalCacheSize, eligibleCacheBytes int64
	var hasEligibleCache bool
	var latestCacheTime time.Time
	var hitPermissionError bool

	allowlistedItems := []string{"WebKitCache", "Cache.db", "Cache.db-wal", "Cache.db-shm", "fsCachedData"}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		name := entry.Name()
		matched := false
		for _, allow := range allowlistedItems {
			if name == allow || strings.HasPrefix(name, allow) {
				matched = true
				break
			}
		}
		if !matched {
			continue // Skip un-allowlisted files
		}

		p := filepath.Join(cacheRoot, name)
		if IsSessionOrProtectedData(p) {
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			if os.IsPermission(err) || isSyscallPermission(err) {
				hitPermissionError = true
			}
			continue
		}

		var sz int64
		if !info.IsDir() {
			sz = info.Size()
		} else {
			dirSz, err := devfs.CalcDirSize(p)
			if err != nil && (os.IsPermission(err) || isSyscallPermission(err)) {
				hitPermissionError = true
			}
			sz = dirSz
		}
		if sz == 0 {
			continue
		}

		totalCacheSize += sz
		if info.ModTime().After(latestCacheTime) {
			latestCacheTime = info.ModTime()
		}
		if maxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleCacheBytes += sz
			hasEligibleCache = true
		}
	}

	if totalCacheSize == 0 && !hitPermissionError {
		return nil, nil
	}

	if latestCacheTime.IsZero() {
		latestCacheTime = time.Now()
	}

	vPath := "browser://safari/caches"
	trg := domain.Target{
		ID:            hashBrowserPath(vPath),
		Name:          "Safari Caches",
		Path:          vPath,
		SizeBytes:     totalCacheSize,
		EligibleBytes: eligibleCacheBytes,
		TTLEligible:   hasEligibleCache,
		Risk:          domain.RiskLow,
		LastAccessed:  latestCacheTime,
		Category:      "browser-cache",
	}

	if isProcessBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	} else if hitPermissionError {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonSystemProtected
		trg.EligibleBytes = 0
		trg.TTLEligible = false
		trg.ProviderMetrics = &domain.ProviderMetrics{
			Partial:    true,
			StatusNote: "permission denied (TCC / Full Disk Access required)",
		}
	}

	// Strictly ZERO ScriptCache targets for Safari (BROW-04, D-08)
	return []domain.Target{trg}, nil
}

// cleanSafari unlinks allowlisted cache leaves under ~/Library/Caches/com.apple.Safari.
// Strictly NEVER touches ~/Library/Safari (BROW-03, D-13).
func cleanSafari(homeDir string) error {
	cacheRoot := filepath.Join(homeDir, "Library", "Caches", "com.apple.Safari")
	allowlistedItems := []string{"WebKitCache", "Cache.db", "Cache.db-wal", "Cache.db-shm", "fsCachedData"}

	for _, item := range allowlistedItems {
		p := filepath.Join(cacheRoot, item)
		if err := VerifyCleanCandidate(p); err != nil {
			continue
		}
		if isLink, err := safety.IsSymlink(p); err == nil && !isLink {
			_ = os.RemoveAll(p)
		}
	}
	return nil
}

func isSyscallPermission(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EACCES || errno == syscall.EPERM
	}
	return false
}
