//go:build windows

package storage

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// freeSpace usa GetDiskFreeSpaceEx. Solo se usa al desarrollar en Windows: en
// produccion el panel corre en Linux dentro del contenedor.
func freeSpace(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("ruta invalida %q: %w", path, err)
	}

	var freeForCaller, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeForCaller, &total, &totalFree); err != nil {
		return 0, fmt.Errorf("consultando espacio libre en %s: %w", path, err)
	}
	return freeForCaller, nil
}
