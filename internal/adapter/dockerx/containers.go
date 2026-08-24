package dockerx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// Runtime implementa app.ContainerRuntime.
type Runtime struct{ c *Client }

func NewRuntime(host string) (*Runtime, error) {
	c, err := New(host)
	if err != nil {
		return nil, err
	}
	return &Runtime{c: c}, nil
}

func (r *Runtime) Ping(ctx context.Context) error { return r.c.Ping(ctx) }

// --- Estructuras de la API de Docker que nos interesan ----------------------

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type createRequest struct {
	Image        string              `json:"Image"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Env          []string            `json:"Env"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	OpenStdin    bool                `json:"OpenStdin"`
	Tty          bool                `json:"Tty"`
	HostConfig   hostConfig          `json:"HostConfig"`
	// NetworkingConfig conecta el contenedor a una red al crearlo.
	//
	// Tiene que ser al CREARLO y no despues: conectar una red a posteriori
	// necesitaria el endpoint /networks, que el proxy de Docker no expone
	// (M-4), y ademas dejaria un hueco en el que el contenedor existe sin la
	// red que se espera que tenga.
	NetworkingConfig *networkingConfig `json:"NetworkingConfig,omitempty"`
}

type networkingConfig struct {
	EndpointsConfig map[string]endpointConfig `json:"EndpointsConfig"`
}

type endpointConfig struct {
	// Aliases son los nombres por los que se le podra llamar dentro de la red.
	Aliases []string `json:"Aliases,omitempty"`
}

type hostConfig struct {
	Binds         []string                 `json:"Binds"`
	PortBindings  map[string][]portBinding `json:"PortBindings"`
	Memory        int64                    `json:"Memory"`
	NanoCPUs      int64                    `json:"NanoCpus"`
	RestartPolicy restartPolicy            `json:"RestartPolicy"`
	LogConfig     logConfig                `json:"LogConfig"`
}

type restartPolicy struct {
	Name string `json:"Name"`
}

type logConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config"`
}

type inspectResponse struct {
	ID    string `json:"Id"`
	State struct {
		Running  bool   `json:"Running"`
		Status   string `json:"Status"`
		ExitCode int    `json:"ExitCode"`
	} `json:"State"`
}

// --- Operaciones ------------------------------------------------------------

func (r *Runtime) Create(ctx context.Context, spec app.ContainerSpec) (string, error) {
	// Si quedo un contenedor con ese nombre de una instancia anterior, se
	// retira: Docker no admite nombres repetidos y la definicion pudo cambiar.
	if id, _ := r.findByName(ctx, spec.Name); id != "" {
		if err := r.Remove(ctx, id); err != nil {
			return "", err
		}
	}

	if err := r.ensureImage(ctx, spec.Image); err != nil {
		return "", err
	}

	proto := spec.Protocol
	if proto == "" {
		proto = "tcp"
	}
	port := fmt.Sprintf("%d/%s", spec.PortIn, proto)

	env := make([]string, 0, len(spec.Env))
	// UID/GID los entiende la imagen de itzg y con ellos elige el usuario al
	// que hace chown de /data. Sin esto elige el suyo y el panel se queda sin
	// permiso de escritura sobre la carpeta de la instancia.
	if spec.UID > 0 {
		env = append(env, "UID="+strconv.Itoa(spec.UID))
	}
	if spec.GID > 0 {
		env = append(env, "GID="+strconv.Itoa(spec.GID))
	}

	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	req := createRequest{
		Image:        spec.Image,
		Cmd:          spec.Cmd,
		Env:          env,
		ExposedPorts: map[string]struct{}{port: {}},
		// stdin abierto y tty apagado: la imagen envia las ordenes al servidor
		// por su entrada estandar, que es como funciona send-command (H-F0-7).
		OpenStdin: true,
		Tty:       false,
		NetworkingConfig: redDe(spec),
		HostConfig: hostConfig{
			Binds:        []string{spec.DataDir + ":/data"},
			PortBindings: map[string][]portBinding{port: {{HostIP: "0.0.0.0", HostPort: strconv.Itoa(spec.PortHost)}}},
			// Limites de M-1. Son un techo, no una reserva: Docker no aparta
			// esta memoria, solo impide superarla.
			Memory:   int64(spec.MemoryMB) * 1024 * 1024,
			NanoCPUs: int64(spec.CPUs * 1e9),
			// Sin reinicio automatico a proposito: si un servidor se cae hay
			// que mirar por que, no dejar que Docker lo levante en bucle
			// consumiendo CPU que necesita la produccion del cliente (D-09).
			RestartPolicy: restartPolicy{Name: "no"},
			// Sin rotacion, los logs pueden llenar el disco; y si el disco se
			// llena, MySQL del cliente deja de escribir (M-2).
			LogConfig: logConfig{
				Type:   "json-file",
				Config: map[string]string{"max-size": "10m", "max-file": "3"},
			},
		},
	}

	var out struct {
		ID string `json:"Id"`
	}
	q := url.Values{"name": {spec.Name}}
	if err := r.c.doJSON(ctx, http.MethodPost, "/containers/create", q, req, &out); err != nil {
		return "", fmt.Errorf("creando el contenedor %s: %w", spec.Name, err)
	}
	return out.ID, nil
}

func (r *Runtime) Start(ctx context.Context, id string) error {
	err := r.c.doJSON(ctx, http.MethodPost, "/containers/"+id+"/start", nil, nil, nil)
	if err != nil {
		// 304 significa "ya estaba arrancado": no es un fallo.
		var apiErr *APIError
		if asAPIError(err, &apiErr) && apiErr.Status == http.StatusNotModified {
			return nil
		}
		return fmt.Errorf("arrancando el contenedor: %w", err)
	}
	return nil
}

// StopAndWait pide la parada y ESPERA a que el contenedor termine de verdad.
//
// Medido en F0 (H-F0-6): `docker stop` retorna aunque el contenedor ya
// estuviera parado, y devolvio 0.086 s cuando el apagado real tardaba ~2 s.
// Fiarse de su retorno lleva a creer que el servidor se apago cuando todavia
// esta guardando el mundo; arrancar otro encima lo corrompe.
func (r *Runtime) StopAndWait(ctx context.Context, id string, timeout time.Duration) error {
	q := url.Values{"t": {strconv.Itoa(int(timeout.Seconds()))}}

	// Docker envia SIGTERM y espera; la imagen lo traduce a un `stop` por la
	// entrada estandar del servidor, que es el apagado limpio. Solo si agota
	// ese plazo manda SIGKILL.
	if err := r.c.doJSON(ctx, http.MethodPost, "/containers/"+id+"/stop", q, nil, nil); err != nil {
		var apiErr *APIError
		if asAPIError(err, &apiErr) &&
			(apiErr.NotFound() || apiErr.Status == http.StatusNotModified) {
			return nil
		}
		return fmt.Errorf("pidiendo la parada: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout(timeout))
	defer cancel()

	wq := url.Values{"condition": {"not-running"}}
	if err := r.c.doJSON(waitCtx, http.MethodPost, "/containers/"+id+"/wait", wq, nil, nil); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("esperando a que se detenga: %w", err)
	}
	return nil
}

func (r *Runtime) Remove(ctx context.Context, id string) error {
	q := url.Values{"force": {"true"}, "v": {"false"}}
	if err := r.c.doJSON(ctx, http.MethodDelete, "/containers/"+id, q, nil, nil); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("borrando el contenedor: %w", err)
	}
	return nil
}

func (r *Runtime) Status(ctx context.Context, id string) (app.ContainerStatus, error) {
	if id == "" {
		return app.ContainerStatus{}, nil
	}

	var insp inspectResponse
	if err := r.c.doJSON(ctx, http.MethodGet, "/containers/"+id+"/json", nil, nil, &insp); err != nil {
		if isNotFound(err) {
			return app.ContainerStatus{}, nil
		}
		return app.ContainerStatus{}, fmt.Errorf("consultando el contenedor: %w", err)
	}

	return app.ContainerStatus{
		Exists:   true,
		Running:  insp.State.Running,
		ExitCode: insp.State.ExitCode,
		Status:   insp.State.Status,
	}, nil
}

// Exec ejecuta una orden dentro del contenedor.
//
// Hace falta para recargar la allow-list sin reiniciar (H-F0-7), lo que obliga
// a matizar M-4: el proxy de Docker no puede prohibir exec del todo; debe
// permitirlo acotado a send-command y solo sobre contenedores de MCVPS.
func (r *Runtime) Exec(ctx context.Context, id string, cmd []string, user string) (string, error) {
	var created struct {
		ID string `json:"Id"`
	}
	req := map[string]any{
		"Cmd":          cmd,
		"AttachStdout": true,
		"AttachStderr": true,
	}
	if user != "" {
		req["User"] = user
	}
	if err := r.c.doJSON(ctx, http.MethodPost, "/containers/"+id+"/exec", nil, req, &created); err != nil {
		return "", fmt.Errorf("preparando la orden %v: %w", cmd, err)
	}

	resp, err := r.c.do(ctx, http.MethodPost, "/exec/"+created.ID+"/start", nil,
		map[string]any{"Detach": false, "Tty": false})
	if err != nil {
		return "", fmt.Errorf("ejecutando la orden %v: %w", cmd, err)
	}
	defer resp.Body.Close()

	out, _ := demux(resp.Body)

	var insp struct {
		ExitCode int  `json:"ExitCode"`
		Running  bool `json:"Running"`
	}
	if err := r.c.doJSON(ctx, http.MethodGet, "/exec/"+created.ID+"/json", nil, nil, &insp); err == nil {
		if !insp.Running && insp.ExitCode != 0 {
			return out, fmt.Errorf("la orden %v salio con codigo %d: %s",
				cmd, insp.ExitCode, strings.TrimSpace(out))
		}
	}
	return out, nil
}

func (r *Runtime) Logs(ctx context.Context, id string, tail int) (string, error) {
	if id == "" {
		return "", nil
	}

	q := url.Values{
		"stdout": {"true"},
		"stderr": {"true"},
		"tail":   {strconv.Itoa(tail)},
	}
	resp, err := r.c.do(ctx, http.MethodGet, "/containers/"+id+"/logs", q, nil)
	if err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("leyendo los logs: %w", err)
	}
	defer resp.Body.Close()

	out, _ := demux(resp.Body)
	return out, nil
}

func (r *Runtime) findByName(ctx context.Context, name string) (string, error) {
	var insp inspectResponse
	if err := r.c.doJSON(ctx, http.MethodGet, "/containers/"+name+"/json", nil, nil, &insp); err != nil {
		if isNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return insp.ID, nil
}

// ensureImage descarga la imagen si falta. La primera vez tarda varios minutos;
// despues es instantaneo.
func (r *Runtime) ensureImage(ctx context.Context, image string) error {
	if err := r.c.doJSON(ctx, http.MethodGet, "/images/"+image+"/json", nil, nil, nil); err == nil {
		return nil
	}

	name, tag, ok := strings.Cut(image, ":")
	if !ok {
		tag = "latest"
	}

	q := url.Values{"fromImage": {name}, "tag": {tag}}
	resp, err := r.c.do(ctx, http.MethodPost, "/images/create", q, nil)
	if err != nil {
		return fmt.Errorf("descargando la imagen %s: %w", image, err)
	}
	defer resp.Body.Close()

	// Hay que consumir la respuesta entera: Docker envia el progreso en
	// streaming y cerrar antes aborta la descarga.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("descargando la imagen %s: %w", image, err)
	}
	return nil
}

// redDe conecta la instancia a la red del panel, si se indico una.
//
// Sin esto el contenedor va a la red por defecto, y entonces el panel no puede
// preguntarle nada: Docker AISLA las redes de usuario entre si -las cadenas
// DOCKER-ISOLATION- y un paquete de un bridge a otro se descarta. El sintoma
// es un servidor que arranca perfectamente y un panel clavado en "arrancando".
//
// Compartiendo red se le pregunta por su nombre, sin NAT, sin salir al host y
// sin depender de reglas de iptables que cualquiera puede recargar.
func redDe(spec app.ContainerSpec) *networkingConfig {
	if spec.Network == "" {
		return nil
	}
	return &networkingConfig{
		EndpointsConfig: map[string]endpointConfig{
			spec.Network: {Aliases: []string{spec.Name}},
		},
	}
}
