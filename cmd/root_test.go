package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRootCmd_FlagsAndProfileLoading(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(homeDir, 0755)
	t.Setenv("HOME", homeDir)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := "scan_paths:\n  - /base/path\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	origProfileName := profileName
	configPath = cfgPath
	profileName = ""
	defer func() {
		configPath = origConfigPath
		profileName = origProfileName
	}()

	// 1. Carregamento sem profile
	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loadConfigWithProfile() unexpected error: %v", err)
	}
	if len(cfg.ScanPaths) != 1 || cfg.ScanPaths[0] != "/base/path" {
		t.Errorf("expected ScanPaths ['/base/path'], got %v", cfg.ScanPaths)
	}

	// 2. Carregamento com profile existente
	profilesDir := filepath.Join(homeDir, ".config", "devclean", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(profilesDir, "test-profile.yaml")
	profileContent := "scan_paths:\n  - /profile/extra\n"
	if err := os.WriteFile(profilePath, []byte(profileContent), 0644); err != nil {
		t.Fatal(err)
	}

	profileName = "test-profile"
	cfgWithProf, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loadConfigWithProfile() with profile error: %v", err)
	}
	// ScanPaths são mesclados com append em MergeProfile
	if len(cfgWithProf.ScanPaths) < 2 {
		t.Errorf("expected at least 2 ScanPaths after profile merge, got %v", cfgWithProf.ScanPaths)
	}

	// 3. Carregamento com profile inexistente
	profileName = "nonexistent-profile"
	_, err = loadConfigWithProfile()
	if err == nil {
		t.Error("expected error for nonexistent profile, got nil")
	}

	// 4. Invocação de rootCmd com --help
	profileName = ""
	var outBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute --help failed: %v", err)
	}

	// 5. Testar função global Execute() com --help
	rootCmd.SetArgs([]string{"--help"})
	if err := Execute(); err != nil {
		t.Fatalf("Execute() with --help failed: %v", err)
	}

	// 6. Testar PersistentPreRunE quando watchDaemon é falso
	if err := rootCmd.PersistentPreRunE(rootCmd, []string{}); err != nil {
		t.Errorf("PersistentPreRunE without watchDaemon failed: %v", err)
	}

	// 7. Validação de registro de flags persistentes
	flags := rootCmd.PersistentFlags()
	requiredFlags := []string{"config", "profile", "verbose", "quiet", "watch-daemon"}
	for _, f := range requiredFlags {
		if flags.Lookup(f) == nil {
			t.Errorf("persistent flag %q not registered on rootCmd", f)
		}
	}
}

func TestRootCmd_ConfigLoadError(t *testing.T) {
	origConfigPath := configPath
	configPath = "/invalid/nonexistent/config.yaml"
	defer func() { configPath = origConfigPath }()

	_, err := loadConfigWithProfile()
	if err == nil {
		t.Error("expected error when loading from invalid configPath")
	}
}
