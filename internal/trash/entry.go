package trash

import "time"

// Entry represents a single trashed item.
type Entry struct {
	ID           string    `json:"id"`
	BatchID      string    `json:"batch_id"`
	OriginalPath string    `json:"original_path"`
	SizeBytes    int64     `json:"size_bytes"`
	Ecosystem    string    `json:"ecosystem"`
	MovedAt      time.Time `json:"moved_at"`
}

// Batch groups entries created in a single clean invocation.
type Batch struct {
	ID        string    `json:"id"`
	Entries   []Entry   `json:"entries"`
	CreatedAt time.Time `json:"created_at"`
	TotalSize int64     `json:"total_size"`
}
