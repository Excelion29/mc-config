//go:build windows

package storage

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// diskUsage usa GetDiskFreeSpaceEx. Solo se usa al desarrollar en Windows: en
// produccion el panel corre en Linux dentro del contenedor.
func diskUsage(path string) (libre, total uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, fmt.Errorf("ruta invalida %q: %w", path, err)
	}

	var disponible, capacidad, libreTotal uint64
	if err := windows.GetDiskFreeSpaceEx(p, &disponible, &capacidad, &libreTotal); err != nil {
		return 0, 0, fmt.Errorf("consultando espacio en %s: %w", path, err)
	}
	return disponible, capacidad, nil
}
