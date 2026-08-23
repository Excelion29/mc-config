//go:build windows

package dockerx

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// dialPipe conecta con el named pipe de Docker Desktop.
//
// Solo se usa al desarrollar en Windows: en produccion el panel corre en Linux
// dentro del contenedor y habla con el socket-proxy por TCP.
func dialPipe(addr string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		return winio.DialPipeContext(ctx, addr)
	}
}

func defaultHost() string { return "npipe:////./pipe/docker_engine" }
