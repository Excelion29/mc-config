//go:build !windows

package dockerx

import (
	"context"
	"fmt"
	"net"
)

// dialPipe no aplica fuera de Windows: los named pipes son de ese sistema.
func dialPipe(addr string) func(context.Context, string, string) (net.Conn, error) {
	return func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("npipe no esta disponible en este sistema: %s", addr)
	}
}

func defaultHost() string { return "unix:///var/run/docker.sock" }
