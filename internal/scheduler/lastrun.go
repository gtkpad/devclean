package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// RecordRun persists a RunResult as JSON in the config directory.
func RecordRun(configDir string, result RunResult) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "last-run.json"), data, 0644)
}

// ReadLastRun reads the last execution result from the config directory.
// Returns nil, nil if the file does not exist.
func ReadLastRun(configDir string) (*RunResult, error) {
	data, err := os.ReadFile(filepath.Join(configDir, "last-run.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result RunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
