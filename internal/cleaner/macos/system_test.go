package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestDiagnosticReports_UserEligible_SystemReviewOnly(t *testing.T) {
	tempDir := t.TempDir()
	userHome := filepath.Join(tempDir, "user_home")
	mockSysDiagDir := filepath.Join(tempDir, "system_reports")
	mockVarLogDir := filepath.Join(tempDir, "system_logs")

	// 1. Setup User Diagnostic Reports: ~/Library/Logs/DiagnosticReports
	userDiagDir := filepath.Join(userHome, "Library", "Logs", "DiagnosticReports")
	if err := os.MkdirAll(userDiagDir, 0o755); err != nil {
		t.Fatalf("failed to create userDiagDir: %v", err)
	}
	userReportFile := filepath.Join(userDiagDir, "app_crash.ips")
	if err := os.WriteFile(userReportFile, make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("failed to create userReportFile: %v", err)
	}

	// 2. Setup System Diagnostic Reports: /Library/Logs/DiagnosticReports
	if err := os.MkdirAll(mockSysDiagDir, 0o755); err != nil {
		t.Fatalf("failed to create mockSysDiagDir: %v", err)
	}
	sysReportFile := filepath.Join(mockSysDiagDir, "kernel_panic.ips")
	if err := os.WriteFile(sysReportFile, make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("failed to create sysReportFile: %v", err)
	}

	// 3. Setup System Logs: /private/var/log
	if err := os.MkdirAll(mockVarLogDir, 0o755); err != nil {
		t.Fatalf("failed to create mockVarLogDir: %v", err)
	}
	sysLogFile := filepath.Join(mockVarLogDir, "install.log")
	if err := os.WriteFile(sysLogFile, make([]byte, 8192), 0o644); err != nil {
		t.Fatalf("failed to create sysLogFile: %v", err)
	}

	mockExec := &cleaner.MockCommandExecutor{}
	ctx := context.Background()

	targets, err := scanSystemLogsAndReportsWithPaths(ctx, mockExec, userHome, mockSysDiagDir, mockVarLogDir)
	if err != nil {
		t.Fatalf("unexpected error scanning system logs and reports: %v", err)
	}

	targetMap := make(map[string]domain.Target)
	for _, target := range targets {
		targetMap[target.ID] = target
	}

	// Verify User Diagnostic Reports: eligible for cleanup
	userTarget, ok := targetMap["macos-user-diagnostic-reports"]
	if !ok {
		t.Fatalf("missing target 'macos-user-diagnostic-reports'")
	}
	if userTarget.ReviewOnly {
		t.Errorf("user diagnostic reports should NOT be ReviewOnly")
	}
	if userTarget.EligibleBytes != 2048 {
		t.Errorf("user diagnostic reports EligibleBytes = %d, want 2048", userTarget.EligibleBytes)
	}
	if userTarget.Category != "macos-diagnostic-reports" {
		t.Errorf("user diagnostic reports Category = %q, want 'macos-diagnostic-reports'", userTarget.Category)
	}

	// Verify System Diagnostic Reports: ReviewOnly with system_protected reason
	sysTarget, ok := targetMap["macos-system-diagnostic-reports"]
	if !ok {
		t.Fatalf("missing target 'macos-system-diagnostic-reports'")
	}
	if !sysTarget.ReviewOnly {
		t.Errorf("system diagnostic reports MUST be ReviewOnly")
	}
	if sysTarget.EligibleBytes != 0 {
		t.Errorf("system diagnostic reports EligibleBytes = %d, want 0", sysTarget.EligibleBytes)
	}
	if sysTarget.ReviewReason != domain.ReviewReasonSystemProtected {
		t.Errorf("system diagnostic reports ReviewReason = %q, want %q", sysTarget.ReviewReason, domain.ReviewReasonSystemProtected)
	}
	if sysTarget.ProviderMetrics == nil || sysTarget.ProviderMetrics.SizeBytes != 4096 {
		t.Errorf("system diagnostic reports ProviderMetrics.SizeBytes = %v, want 4096", sysTarget.ProviderMetrics)
	}

	// Verify System Logs: ReviewOnly with system_protected reason
	logTarget, ok := targetMap["macos-system-logs"]
	if !ok {
		t.Fatalf("missing target 'macos-system-logs'")
	}
	if !logTarget.ReviewOnly {
		t.Errorf("system logs MUST be ReviewOnly")
	}
	if logTarget.EligibleBytes != 0 {
		t.Errorf("system logs EligibleBytes = %d, want 0", logTarget.EligibleBytes)
	}
	if logTarget.ReviewReason != domain.ReviewReasonSystemProtected {
		t.Errorf("system logs ReviewReason = %q, want %q", logTarget.ReviewReason, domain.ReviewReasonSystemProtected)
	}
	if logTarget.ProviderMetrics == nil || logTarget.ProviderMetrics.SizeBytes != 8192 {
		t.Errorf("system logs ProviderMetrics.SizeBytes = %v, want 8192", logTarget.ProviderMetrics)
	}
}

func TestSystemLogs_GracefulDegradationTCC(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create readable file (1024 bytes)
	readableFile := filepath.Join(tempDir, "readable.log")
	if err := os.WriteFile(readableFile, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("failed to write readable file: %v", err)
	}

	// 2. Create restricted subdirectory (permission denied simulation)
	restrictedDir := filepath.Join(tempDir, "restricted")
	if err := os.MkdirAll(restrictedDir, 0o755); err != nil {
		t.Fatalf("failed to create restrictedDir: %v", err)
	}
	hiddenFile := filepath.Join(restrictedDir, "secure.log")
	if err := os.WriteFile(hiddenFile, make([]byte, 512), 0o644); err != nil {
		t.Fatalf("failed to write hidden file: %v", err)
	}

	// Lock the directory to simulate TCC or permission denial
	if err := os.Chmod(restrictedDir, 0o000); err != nil {
		t.Fatalf("failed to chmod restrictedDir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(restrictedDir, 0o755)
	})

	ctx := context.Background()
	totalSize, partial, note := scanDirectoryWithTCCFallback(ctx, tempDir)

	if totalSize != 1024 {
		t.Errorf("totalSize = %d, want 1024 readable bytes", totalSize)
	}
	if !partial {
		t.Errorf("partial = %v, want true (graceful degradation)", partial)
	}
	if note != "permission denied (TCC / root required)" {
		t.Errorf("note = %q, want 'permission denied (TCC / root required)'", note)
	}
}

func TestSystemLogs_NonExistentDirectories(t *testing.T) {
	mockExec := &cleaner.MockCommandExecutor{}
	ctx := context.Background()

	targets, err := scanSystemLogsAndReportsWithPaths(ctx, mockExec, "/nonexistent/home", "/nonexistent/sysdiag", "/nonexistent/varlog")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("len(targets) = %d, want 0 for non-existent directories", len(targets))
	}
}
