package trash

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Manifest tracks all batches in the trash.
type Manifest struct {
	Batches []Batch `json:"batches"`
}

// LoadManifest reads the manifest from path. Returns an empty manifest if the file is missing.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &Manifest{}, nil
	}
	return &m, nil
}

// SaveManifest writes the manifest atomically using temp+rename.
func SaveManifest(path string, m *Manifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FindBatch returns the batch with the given ID or nil.
func (m *Manifest) FindBatch(id string) *Batch {
	for i := range m.Batches {
		if m.Batches[i].ID == id {
			return &m.Batches[i]
		}
	}
	return nil
}

// FindEntry returns the entry and its parent batch, or nil if not found.
func (m *Manifest) FindEntry(id string) (*Entry, *Batch) {
	for i := range m.Batches {
		for j := range m.Batches[i].Entries {
			if m.Batches[i].Entries[j].ID == id {
				return &m.Batches[i].Entries[j], &m.Batches[i]
			}
		}
	}
	return nil, nil
}

// RemoveBatch removes a batch by ID from the manifest.
func (m *Manifest) RemoveBatch(id string) {
	for i := range m.Batches {
		if m.Batches[i].ID == id {
			m.Batches = append(m.Batches[:i], m.Batches[i+1:]...)
			return
		}
	}
}

// RemoveEntry removes a single entry by ID. Removes the parent batch if it becomes empty.
func (m *Manifest) RemoveEntry(id string) {
	for i := range m.Batches {
		for j := range m.Batches[i].Entries {
			if m.Batches[i].Entries[j].ID == id {
				m.Batches[i].TotalSize -= m.Batches[i].Entries[j].SizeBytes
				m.Batches[i].Entries = append(m.Batches[i].Entries[:j], m.Batches[i].Entries[j+1:]...)
				if len(m.Batches[i].Entries) == 0 {
					m.Batches = append(m.Batches[:i], m.Batches[i+1:]...)
				}
				return
			}
		}
	}
}

// TotalSize returns the sum of all entry sizes in the manifest.
func (m *Manifest) TotalSize() int64 {
	var total int64
	for i := range m.Batches {
		total += m.Batches[i].TotalSize
	}
	return total
}

// LastBatch returns the most recently added batch, or nil if empty.
func (m *Manifest) LastBatch() *Batch {
	if len(m.Batches) == 0 {
		return nil
	}
	return &m.Batches[len(m.Batches)-1]
}
