package fs

import (
	"os"
	"path/filepath"
)

// DiskSpace armazena métricas de capacidade e espaço livre no filesystem.
type DiskSpace struct {
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

// GetDiskSpace obtém o espaço em disco do filesystem que contém o caminho indicado.
// Se o caminho não existir (ex.: acabou de ser limpo), navega deterministicamente até o
// primeiro diretório pai existente para evitar erros de ENOENT.
func GetDiskSpace(path string) (DiskSpace, error) {
	target := filepath.Clean(path)
	for {
		if _, err := os.Stat(target); err == nil {
			break
		}
		parent := filepath.Dir(target)
		if parent == target || parent == "." {
			target = "."
			break
		}
		target = parent
	}
	return getPlatformDiskSpace(target)
}
