package domain

import (
	"strings"
	"time"
)

// InstanceState es el estado de un servidor.
//
// Los estados intermedios existen porque arrancar y parar no son instantaneos:
// medido en F0, el arranque tarda ~5 s y el apagado limpio ~2 s con el mundo
// vacio. Sin ellos, dos peticiones seguidas darian ordenes contradictorias al
// mismo contenedor.
type InstanceState string

const (
	StateStopped  InstanceState = "stopped"
	StateStarting InstanceState = "starting"
	StateRunning  InstanceState = "running"
	StateStopping InstanceState = "stopping"
	// StateFailed marca que algo salio mal y hace falta mirar los logs. No se
	// arranca ni se para automaticamente desde aqui: alguien tiene que ver que
	// paso antes de volver a tocarlo.
	StateFailed InstanceState = "failed"
)

func (s InstanceState) Label() string {
	switch s {
	case StateStopped:
		return "Detenida"
	case StateStarting:
		return "Arrancando"
	case StateRunning:
		return "Activa"
	case StateStopping:
		return "Parando"
	case StateFailed:
		return "Con fallo"
	}
	return string(s)
}

// Busy indica que hay una operacion en curso y no se debe pedir otra.
func (s InstanceState) Busy() bool {
	return s == StateStarting || s == StateStopping
}

// Instance es un servidor concreto: una version, un mapa y su configuracion.
type Instance struct {
	ID   int64
	Name string
	// Slug se usa para el nombre del contenedor y de la carpeta. Solo minusculas,
	// numeros y guiones: acaba en rutas y en argumentos de Docker.
	Slug        string
	Edition     Edition
	Version     string
	WorldID       int64
	WorldName     string
	LevelName   string
	ContainerID string
	Port        int
	State       InstanceState
	MemoryMB    int
	CPUs        float64
	CreatedAt   time.Time
	LastStarted *time.Time
}

// CanStart indica si tiene sentido pedir el arranque.
func (i *Instance) CanStart() bool {
	return i != nil && (i.State == StateStopped || i.State == StateFailed)
}

// CanStop indica si tiene sentido pedir la parada.
func (i *Instance) CanStop() bool {
	return i != nil && (i.State == StateRunning || i.State == StateFailed)
}

// Slugify convierte un nombre en algo usable como carpeta y nombre de
// contenedor. Docker acepta un juego de caracteres limitado, y el resultado
// acaba tambien en rutas del sistema de archivos.
func Slugify(name string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ñ", "n", "Ü", "u",
	)
	name = replacer.Replace(strings.ToLower(strings.TrimSpace(name)))

	var b strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case !prevDash && b.Len() > 0:
			b.WriteByte('-')
			prevDash = true
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	return slug
}

// Limites por defecto de una instancia (M-1).
//
// Medido en F0 (H-F0-10): el servidor con el mapa cargado usa ~342 MiB. El
// limite es un techo, no una reserva: Docker no aparta esa memoria, solo impide
// superarla. Dejarlo holgado no le quita nada a la produccion del cliente y
// protege del escenario que preocupaba en D-09.
const (
	DefaultMemoryMB = 3072
	DefaultCPUs     = 2.0

	// PortBedrock es el puerto estandar. Por D-02 solo hay un servidor
	// encendido a la vez, asi que no hay que repartir puertos.
	PortBedrock = 19132
)
