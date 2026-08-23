//go:build !windows

package storage

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// diskUsage usa statfs. Se cuentan los bloques disponibles para un usuario sin
// privilegios (Bavail), no los libres totales (Bfree): en ext4 una parte esta
// reservada para root y no la podriamos usar.
//
// El total se calcula sobre esa misma base -Bavail + usados- y no sobre Blocks,
// para que el porcentaje no mienta: si se dividiera lo disponible entre el
// tamano bruto del disco, el panel diria "queda un 5%" cuando en realidad ya no
// queda nada utilizable.
func diskUsage(path string) (libre, total uint64, err error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, fmt.Errorf("consultando espacio en %s: %w", path, err)
	}
	bloque := uint64(st.Bsize)
	libre = st.Bavail * bloque
	usado := (st.Blocks - st.Bfree) * bloque
	return libre, libre + usado, nil
}
