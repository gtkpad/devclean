package terraform

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestTerraformCleaner_Name(t *testing.T) {
	c := New([]string{})
	if c.Name() != "terraform" {
		t.Fatalf("expected name 'terraform', got %q", c.Name())
	}
}

func TestTerraformCleaner_IsInstalled_LookPath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("TF_PLUGIN_CACHE_DIR", "")

	// Create fake bin dir with 'tofu' executable
	binDir := filepath.Join(tmpHome, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create binDir: %v", err)
	}
	tofuBin := filepath.Join(binDir, "tofu")
	if err := os.WriteFile(tofuBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("failed to write mock tofu bin: %v", err)
	}

	t.Setenv("PATH", binDir)

	c := New([]string{})
	if !c.IsInstalled() {
		t.Error("expected IsInstalled to be true when 'tofu' is in PATH")
	}

	// Now set PATH to empty without any tf files in scanPaths
	t.Setenv("PATH", "")
	cEmpty := New([]string{tmpHome})
	if cEmpty.IsInstalled() {
		t.Error("expected IsInstalled to be false when PATH is empty and no .tf files exist")
	}
}

func TestTerraformCleaner_IsInstalled_TfFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("TF_PLUGIN_CACHE_DIR", "")

	cWithoutTf := New([]string{tmp})
	if cWithoutTf.IsInstalled() {
		t.Error("expected IsInstalled to be false without .tf files")
	}

	// Create a .tf file in scanPaths
	tfFile := filepath.Join(tmp, "main.tf")
	if err := os.WriteFile(tfFile, []byte("provider \"aws\" {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write tfFile: %v", err)
	}

	cWithTf := New([]string{tmp})
	if !cWithTf.IsInstalled() {
		t.Error("expected IsInstalled to be true when .tf files exist in scanPaths (D-10)")
	}
}

func TestTerraformCleaner_Scan_WithTfFiles(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "infra")
	tfDir := filepath.Join(proj, ".terraform")

	if err := os.MkdirAll(tfDir, 0o755); err != nil {
		t.Fatalf("failed to create .terraform dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "main.tf"), []byte("resource \"null_resource\" \"x\" {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	// Remote backend state in .terraform/terraform.tfstate (safe to clean)
	remoteState := `{
  "version": 3,
  "backend": {
    "type": "s3",
    "config": {
      "bucket": "my-bucket",
      "key": "infra.tfstate"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfstate"), []byte(remoteState), 0o644); err != nil {
		t.Fatalf("failed to write remote state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tfDir, "providers.bin"), []byte("provider cache data"), 0o644); err != nil {
		t.Fatalf("failed to write providers.bin: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var found *domain.Target
	for i := range targets {
		if targets[i].Path == tfDir {
			found = &targets[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("expected .terraform directory %s to be found", tfDir)
	}
	if found.Category != "terraform-local" {
		t.Errorf("expected category 'terraform-local', got %q", found.Category)
	}
	if found.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium', got %q", found.Risk)
	}
	if found.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", found.SizeBytes)
	}
	if found.ID == "" {
		t.Error("expected non-empty target ID")
	}
}

func TestTerraformCleaner_Scan_IgnoreWithoutTfFiles(t *testing.T) {
	tmp := t.TempDir()
	genericProj := filepath.Join(tmp, "generic_repo")
	tfDir := filepath.Join(genericProj, ".terraform")

	if err := os.MkdirAll(tfDir, 0o755); err != nil {
		t.Fatalf("failed to create .terraform dir: %v", err)
	}
	// Generic files only - no .tf files
	if err := os.WriteFile(filepath.Join(genericProj, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tfDir, "dummy"), []byte("dummy data"), 0o644); err != nil {
		t.Fatalf("failed to write dummy: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == tfDir {
			t.Fatalf(".terraform directory %s should NOT have been claimed without .tf files", tfDir)
		}
	}
}

func TestTerraformCleaner_Scan_SkipOnActiveLock(t *testing.T) {
	tmp := t.TempDir()

	// Proj 1: active lock in parent directory
	proj1 := filepath.Join(tmp, "proj_parent_lock")
	tfDir1 := filepath.Join(proj1, ".terraform")
	if err := os.MkdirAll(tfDir1, 0o755); err != nil {
		t.Fatalf("failed to create tfDir1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj1, "main.tf"), []byte("resource \"x\" {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj1, ".terraform.tfstate.lock.info"), []byte("{\"ID\": \"lock-1\"}"), 0o644); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	// Proj 2: active lock inside .terraform/
	proj2 := filepath.Join(tmp, "proj_inner_lock")
	tfDir2 := filepath.Join(proj2, ".terraform")
	if err := os.MkdirAll(tfDir2, 0o755); err != nil {
		t.Fatalf("failed to create tfDir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj2, "main.tf"), []byte("resource \"y\" {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tfDir2, ".terraform.tfstate.lock.info"), []byte("{\"ID\": \"lock-2\"}"), 0o644); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == tfDir1 {
			t.Fatalf("tfDir1 %s MUST be skipped when active lock exists in parent dir (D-09)", tfDir1)
		}
		if target.Path == tfDir2 {
			t.Fatalf("tfDir2 %s MUST be skipped when active lock exists inside .terraform (D-09)", tfDir2)
		}
	}
}

func TestTerraformCleaner_Scan_SkipOnLocalState(t *testing.T) {
	tmp := t.TempDir()

	// Proj 1: .terraform/terraform.tfstate specifies local backend
	proj1 := filepath.Join(tmp, "proj_local_backend")
	tfDir1 := filepath.Join(proj1, ".terraform")
	if err := os.MkdirAll(tfDir1, 0o755); err != nil {
		t.Fatalf("failed to create tfDir1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj1, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	localBackendState := `{
  "version": 3,
  "backend": {
    "type": "local"
  }
}`
	if err := os.WriteFile(filepath.Join(tfDir1, "terraform.tfstate"), []byte(localBackendState), 0o644); err != nil {
		t.Fatalf("failed to write local backend state: %v", err)
	}

	// Proj 2: terraform.tfstate in project root contains local resources
	proj2 := filepath.Join(tmp, "proj_root_state")
	tfDir2 := filepath.Join(proj2, ".terraform")
	if err := os.MkdirAll(tfDir2, 0o755); err != nil {
		t.Fatalf("failed to create tfDir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj2, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	rootStateContent := `{
  "version": 4,
  "resources": [
    {
      "mode": "managed",
      "type": "aws_instance",
      "name": "web"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(proj2, "terraform.tfstate"), []byte(rootStateContent), 0o644); err != nil {
		t.Fatalf("failed to write root state: %v", err)
	}

	// Proj 3: .terraform/terraform.tfstate exists without any "backend" field (unconfigured/local)
	proj3 := filepath.Join(tmp, "proj_no_backend_field")
	tfDir3 := filepath.Join(proj3, ".terraform")
	if err := os.MkdirAll(tfDir3, 0o755); err != nil {
		t.Fatalf("failed to create tfDir3: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj3, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tfDir3, "terraform.tfstate"), []byte("{\"version\": 3}"), 0o644); err != nil {
		t.Fatalf("failed to write state without backend: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == tfDir1 {
			t.Fatalf("tfDir1 %s MUST be skipped when local backend is configured (D-09)", tfDir1)
		}
		if target.Path == tfDir2 {
			t.Fatalf("tfDir2 %s MUST be skipped when local state file with resources exists in project root (D-09)", tfDir2)
		}
		if target.Path == tfDir3 {
			t.Fatalf("tfDir3 %s MUST be skipped when terraform.tfstate lacks backend field (D-09)", tfDir3)
		}
	}
}

func TestTerraformCleaner_Scan_GlobalPluginCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Custom TF_PLUGIN_CACHE_DIR
	customCache := filepath.Join(tmpHome, "custom_plugin_cache")
	if err := os.MkdirAll(customCache, 0o755); err != nil {
		t.Fatalf("failed to create custom_plugin_cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customCache, "plugin.zip"), []byte("plugin"), 0o644); err != nil {
		t.Fatalf("failed to write plugin file: %v", err)
	}
	t.Setenv("TF_PLUGIN_CACHE_DIR", customCache)

	c := New([]string{})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundCustom *domain.Target
	for i := range targets {
		if targets[i].Path == customCache {
			foundCustom = &targets[i]
			break
		}
	}

	if foundCustom == nil {
		t.Fatalf("expected custom plugin cache %s to be found", customCache)
	}
	if foundCustom.Category != "terraform-plugin-cache" {
		t.Errorf("expected category 'terraform-plugin-cache', got %q", foundCustom.Category)
	}
	if foundCustom.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundCustom.Risk)
	}
	if foundCustom.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundCustom.SizeBytes)
	}

	// Default ~/.terraform.d/plugin-cache fallback
	t.Setenv("TF_PLUGIN_CACHE_DIR", "")
	defaultCache := filepath.Join(tmpHome, ".terraform.d", "plugin-cache")
	if err := os.MkdirAll(defaultCache, 0o755); err != nil {
		t.Fatalf("failed to create defaultCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultCache, "provider_linux.zip"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write provider file: %v", err)
	}

	targets, err = c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundDefault *domain.Target
	for i := range targets {
		if targets[i].Path == defaultCache {
			foundDefault = &targets[i]
			break
		}
	}

	if foundDefault == nil {
		t.Fatalf("expected default plugin cache %s to be found", defaultCache)
	}
	if foundDefault.Category != "terraform-plugin-cache" {
		t.Errorf("expected category 'terraform-plugin-cache', got %q", foundDefault.Category)
	}
	if foundDefault.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundDefault.Risk)
	}
}

func TestTerraformCleaner_Clean(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target_tf_dir")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "plugin"), []byte("bin"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	c := New([]string{})
	target := domain.Target{
		ID:       "tf-clean-id",
		Name:     "target_tf_dir",
		Path:     targetDir,
		Category: "terraform-local",
		Risk:     domain.RiskMedium,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("expected targetDir to be removed, got err: %v", err)
	}
}

func TestTerraformCleaner_Clean_Error(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "locked_parent")
	childDir := filepath.Join(parentDir, "target")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(parentDir, 0o777)
	})

	if err := os.Chmod(parentDir, 0o555); err != nil {
		t.Fatalf("failed to chmod parent: %v", err)
	}

	c := New([]string{})
	target := domain.Target{Path: childDir}
	err := c.Clean(context.Background(), []domain.Target{target})
	if err == nil {
		t.Error("expected error when removing child of read-only parent directory, got nil")
	}
}

func TestTerraformCleaner_Scan_SymlinkAndExclusions(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_infra_proj")
	tfDir := filepath.Join(proj, ".terraform")
	if err := os.MkdirAll(tfDir, 0o755); err != nil {
		t.Fatalf("failed to create tfDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}
	// Remote backend so it would otherwise be eligible
	remoteState := `{"version": 3, "backend": {"type": "gcs"}}`
	if err := os.WriteFile(filepath.Join(tfDir, "terraform.tfstate"), []byte(remoteState), 0o644); err != nil {
		t.Fatalf("failed to write remote state: %v", err)
	}

	// Symlink pointing to proj
	linkDir := filepath.Join(tmp, "link_to_infra")
	if err := os.Symlink(proj, linkDir); err != nil {
		t.Skip("symlinks not supported")
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{"my_infra_proj/**"},
	}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == tfDir {
			t.Errorf("tfDir %s should have been excluded by pattern", tfDir)
		}
	}
}

func TestTerraformCleaner_Scan_GitSkipped(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	tfInGit := filepath.Join(gitDir, ".terraform")
	if err := os.MkdirAll(tfInGit, 0o755); err != nil {
		t.Fatalf("failed to create tfInGit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "main.tf"), []byte("terraform {}\n"), 0o644); err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == tfInGit {
			t.Errorf(".git internal .terraform dir should have been skipped")
		}
	}
}

func TestTerraformCleaner_Scan_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New([]string{t.TempDir()})
	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
