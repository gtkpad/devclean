package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const FileName = "history.jsonl"

// Entry represents a single cleanup run record.
type Entry struct {
	Timestamp  time.Time     `json:"timestamp"`
	Targets    int           `json:"targets"`
	Paths      []string      `json:"paths,omitempty"`
	FreedBytes int64         `json:"freed_bytes"`
	Ecosystems []string      `json:"ecosystems"`
	Duration   time.Duration `json:"duration"`
	Mode       string        `json:"mode"`
	BatchID    string        `json:"batch_id,omitempty"`
}

// FilterOpts controls which entries are returned by ReadFiltered.
type FilterOpts struct {
	Since     time.Time
	Ecosystem string
}

// Append adds a single entry to the JSONL file.
func Append(dataDir string, entry Entry) error {
	path := filepath.Join(dataDir, FileName)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// Read returns all entries from the history file.
func Read(dataDir string) ([]Entry, error) {
	path := filepath.Join(dataDir, FileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// ReadFiltered returns entries matching the filter options.
func ReadFiltered(dataDir string, opts FilterOpts) ([]Entry, error) {
	entries, err := Read(dataDir)
	if err != nil {
		return nil, err
	}

	var filtered []Entry
	for _, e := range entries {
		if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
			continue
		}
		if opts.Ecosystem != "" {
			found := false
			for _, eco := range e.Ecosystems {
				if eco == opts.Ecosystem {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

// Truncate removes entries older than maxAge. Rewrites the file atomically.
func Truncate(dataDir string, maxAge time.Duration) error {
	entries, err := Read(dataDir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-maxAge)
	var kept []Entry
	for _, e := range entries {
		if !e.Timestamp.Before(cutoff) {
			kept = append(kept, e)
		}
	}

	path := filepath.Join(dataDir, FileName)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	for _, e := range kept {
		data, err := json.Marshal(e)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		data = append(data, '\n')
		if _, err := f.Write(data); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	f.Close()
	return os.Rename(tmp, path)
}
