//go:build !windows

package storage

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// freeSpace usa statfs. Se cuentan los bloques disponibles para un usuario sin
// privilegios (Bavail), no los libres totales (Bfree): en ext4 una parte esta
// reservada para root y no la podriamos usar.
func freeSpace(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("consultando espacio libre en %s: %w", path, err)
	}
	return st.Bavail * uint64(st.Bsize), nil
}
