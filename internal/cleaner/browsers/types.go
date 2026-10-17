package browsers

// BrowserEngine categorizes rendering engines for specialized cache structures.
type BrowserEngine string

const (
	EngineChromium BrowserEngine = "chromium"
	EngineGecko    BrowserEngine = "gecko"
	EngineWebKit   BrowserEngine = "webkit"
)

// BrowserDef represents declarative identification and traversal paths for a browser.
type BrowserDef struct {
	ID             string
	Name           string
	Engine         BrowserEngine
	DarwinOnly     bool
	HasScriptCache bool
	ProcessNames   map[string][]string // "darwin" -> [...], "linux" -> [...]
	AppSupportDir  func(home, goos string) string
	CacheDir       func(home, goos string) string
}

// ProfileInfo holds resolved paths for an individual user profile.
type ProfileInfo struct {
	Name           string
	AppSupportPath string
	CachePath      string
}

// SubVolumeMetrics aggregates byte sizes across cache sub-categories.
type SubVolumeMetrics struct {
	NetworkBytes      int64
	GPUBytes          int64
	CodeCacheBytes    int64
	CacheStorageBytes int64
	ScriptCacheBytes  int64
}
