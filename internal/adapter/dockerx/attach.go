package dockerx

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SendStdin escribe en la entrada estandar del contenedor.
//
// Sustituye a `exec send-command`, que resulto fragil: busca el proceso del
// servidor recorriendo /proc y comparando el nombre del binario. Ese nombre
// lleva la version pegada (`bedrock_server-1.26.44.3`), asi que la busqueda
// falla; y ademas depende del usuario que ejecute el exec y de que se puedan
// leer los /proc de otros procesos.
//
// Escribir en stdin no depende de nada de eso: es donde el servidor lee sus
// ordenes de todos modos. Los contenedores se crean con OpenStdin=true
// precisamente para esto.
//
// /attach no es HTTP normal. Docker responde 101 Switching Protocols y a partir
// de ahi la conexion es un flujo de bytes en crudo, asi que hay que hablarla a
// mano en vez de usar http.Client.
func (c *Client) SendStdin(ctx context.Context, id, input string) error {
	if !strings.HasSuffix(input, "\n") {
		// Sin salto de linea el servidor no considera terminada la orden y se
		// queda esperando el resto.
		input += "\n"
	}

	conn, err := c.dial(ctx)
	if err != nil {
		return fmt.Errorf("conectando con Docker: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(15 * time.Second))
	}

	host := strings.TrimPrefix(c.base, "http://")
	peticion := fmt.Sprintf(
		"POST /containers/%s/attach?stream=1&stdin=1 HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Connection: Upgrade\r\n"+
			"Upgrade: tcp\r\n"+
			"Content-Length: 0\r\n"+
			"\r\n", id, host)

	if _, err := conn.Write([]byte(peticion)); err != nil {
		return fmt.Errorf("enviando la peticion de attach: %w", err)
	}

	// Hay que consumir la respuesta antes de escribir: si no, los bytes de la
	// orden se mezclarian con las cabeceras que Docker todavia esta enviando.
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return fmt.Errorf("leyendo la respuesta de attach: %w", err)
	}
	defer resp.Body.Close()

	// 101 es lo esperado; 200 tambien vale, algunas versiones no negocian.
	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("attach devolvio %s", resp.Status)
	}

	if _, err := conn.Write([]byte(input)); err != nil {
		return fmt.Errorf("escribiendo la orden: %w", err)
	}
	return nil
}

// SendStdin en el Runtime, para que lo use el ServerFlavor.
func (r *Runtime) SendStdin(ctx context.Context, id, input string) error {
	if id == "" {
		return nil
	}
	return r.c.SendStdin(ctx, id, input)
}
