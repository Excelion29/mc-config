// Package dockerx implementa app.ContainerRuntime hablando directamente con la
// API HTTP de Docker.
//
// Se descarto el SDK oficial: arrastraba 267 paquetes de dependencia y forzaba
// versiones de otras librerias del proyecto. Aqui solo hacen falta ocho
// endpoints, y en produccion no hablamos con Docker sino con un
// docker-socket-proxy (M-4), que expone exactamente esta misma API por HTTP.
// El socket de Docker equivale a root sobre toda la maquina, incluidos los
// contenedores del cliente (D-09), asi que cuanto mas fina sea la superficie,
// mejor.
package dockerx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client habla con el motor de Docker.
type Client struct {
	http *http.Client
	// dial abre una conexion cruda con el motor. Hace falta para /attach, que
	// no es HTTP normal: Docker responde 101 y a partir de ahi la conexion
	// pasa a ser un flujo de bytes hacia la entrada estandar del contenedor.
	dial func(context.Context) (net.Conn, error)
	// base es el prefijo de las URL. Con socket unix o named pipe el host es
	// ficticio, porque la conexion ya esta dirigida por el dialer.
	base string
}

// New crea un cliente.
//
// host admite:
//   - ""                        -> autodeteccion segun el sistema
//   - "unix:///var/run/docker.sock"
//   - "npipe:////./pipe/docker_engine"
//   - "tcp://host:2375"         -> el docker-socket-proxy en produccion
func New(host string) (*Client, error) {
	if host == "" {
		host = defaultHost()
	}

	scheme, addr, ok := strings.Cut(host, "://")
	if !ok {
		return nil, fmt.Errorf("DOCKER_HOST invalido: %q", host)
	}

	transport := &http.Transport{}
	base := "http://docker"

	var dial func(context.Context) (net.Conn, error)

	switch scheme {
	case "unix":
		dial = func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", addr)
		}
	case "npipe":
		pipe := dialPipe(addr)
		dial = func(ctx context.Context) (net.Conn, error) { return pipe(ctx, "", "") }
	case "tcp", "http":
		base = "http://" + addr
		dial = func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		}
	default:
		return nil, fmt.Errorf("esquema no soportado: %q", scheme)
	}

	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dial(ctx)
	}

	return &Client{
		dial: dial,
		// Sin timeout global: hay peticiones largas por naturaleza, como
		// descargar la imagen o esperar a que un contenedor se detenga. El
		// plazo lo pone el contexto de cada llamada.
		http: &http.Client{Transport: transport},
		base: base,
	}, nil
}

// Ping comprueba que el motor responde, para fallar al arrancar y no la primera
// vez que alguien pulse un boton.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/_ping", nil, nil)
	if err != nil {
		return fmt.Errorf("Docker no responde: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// do hace una peticion. query puede ser nil; body se envia como JSON.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("preparando la peticion: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, newAPIError(resp)
	}
	return resp, nil
}

// doJSON hace la peticion y decodifica la respuesta.
func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	resp, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("leyendo la respuesta de %s: %w", path, err)
	}
	return nil
}

// APIError es un error devuelto por Docker.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("docker: %s (HTTP %d)", e.Message, e.Status)
}

// NotFound distingue "no existe" de un fallo real. Se consulta mucho: borrar o
// inspeccionar algo que ya no esta no es un error para nosotros.
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

func newAPIError(resp *http.Response) *APIError {
	var payload struct {
		Message string `json:"message"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	json.Unmarshal(raw, &payload)

	msg := payload.Message
	if msg == "" {
		msg = strings.TrimSpace(string(raw))
	}
	if msg == "" {
		msg = resp.Status
	}
	return &APIError{Status: resp.StatusCode, Message: msg}
}

func isNotFound(err error) bool {
	var apiErr *APIError
	if ok := asAPIError(err, &apiErr); ok {
		return apiErr.NotFound()
	}
	return false
}

func asAPIError(err error, target **APIError) bool {
	for err != nil {
		if e, ok := err.(*APIError); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// demux separa stdout y stderr del flujo multiplexado de Docker.
//
// Sin TTY, Docker antepone a cada fragmento una cabecera de 8 bytes: 1 byte de
// canal, 3 de relleno y 4 con el tamano en big-endian. Leer el flujo sin
// deshacer eso mezcla bytes de control con el texto.
func demux(r io.Reader) (string, error) {
	var out strings.Builder
	header := make([]byte, 8)

	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return out.String(), nil
			}
			return out.String(), err
		}

		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}
		// Tope por si la cabecera viene corrupta y anuncia un tamano absurdo.
		if size > 8<<20 {
			return out.String(), fmt.Errorf("fragmento de %d bytes, demasiado grande", size)
		}

		chunk := make([]byte, size)
		if _, err := io.ReadFull(r, chunk); err != nil {
			out.Write(chunk)
			return out.String(), nil
		}
		out.Write(chunk)
	}
}

// waitTimeout suma margen al plazo de Docker: la espera no debe vencer antes
// que el propio apagado que estamos esperando.
func waitTimeout(d time.Duration) time.Duration { return d + 15*time.Second }
