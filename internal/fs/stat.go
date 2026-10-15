package fs

import "os"

// statPath performs an os.Stat. Separated for testability.
func statPath(path string) error {
	_, err := os.Stat(path)
	return err
}

// DirExists verifica se o caminho existe fisicamente e é um diretório regular ou resolvido.
// Retorna false se o caminho for vazio, se não existir, ou se for um arquivo regular.
func DirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

