package app

import (
	"context"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// InstanceRepo persiste las instancias de servidor.
type InstanceRepo interface {
	Create(ctx context.Context, i *domain.Instance) (int64, error)
	ByID(ctx context.Context, id int64) (*domain.Instance, error)
	BySlug(ctx context.Context, slug string) (*domain.Instance, error)
	List(ctx context.Context) ([]domain.Instance, error)
	// Running devuelve la instancia encendida, si la hay. Por D-02 solo puede
	// haber una.
	Running(ctx context.Context) (*domain.Instance, error)
	SetState(ctx context.Context, id int64, state domain.InstanceState) error
	SetContainer(ctx context.Context, id int64, containerID string) error
	MarkStarted(ctx context.Context, id int64, at time.Time) error
	Delete(ctx context.Context, id int64) error
	CountByMap(ctx context.Context, mapID int64) (int, error)
}

// ContainerSpec describe el contenedor a crear. Lo rellena el ServerFlavor de
// cada edicion; el runtime solo lo ejecuta.
type ContainerSpec struct {
	Name  string
	Image string
	// Cmd solo se indica cuando la imagen no trae entrypoint propio. La de
	// Bedrock si lo trae, asi que queda vacio.
	Cmd      []string
	Env      map[string]string
	DataDir  string // ruta en el host que se monta en /data
	PortHost int
	PortIn   int
	Protocol string // "udp" o "tcp"
	MemoryMB int
	CPUs     float64
}

// ContainerStatus es lo que el runtime sabe del contenedor.
type ContainerStatus struct {
	Exists   bool
	Running  bool
	ExitCode int
	Status   string
}

// ContainerRuntime abstrae Docker.
//
// Es un puerto por dos razones. La primera es la de siempre: los casos de uso
// no deberian saber de Docker. La segunda es de seguridad (M-4): en produccion
// detras hay un proxy con lista blanca, no el socket crudo, porque el socket de
// Docker equivale a root sobre toda la maquina, incluidos los contenedores del
// cliente (D-09).
type ContainerRuntime interface {
	Create(ctx context.Context, spec ContainerSpec) (string, error)
	Start(ctx context.Context, id string) error
	// StopAndWait pide una parada limpia y ESPERA a que el contenedor llegue a
	// "exited". Medido en F0 (H-F0-6): `docker stop` retorna aunque no haya
	// pasado nada, asi que fiarse de su retorno lleva a creer que se apago
	// cuando no. Un corte a lo bruto corrompe el mundo.
	StopAndWait(ctx context.Context, id string, timeout time.Duration) error
	Remove(ctx context.Context, id string) error
	Status(ctx context.Context, id string) (ContainerStatus, error)
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	Logs(ctx context.Context, id string, tail int) (string, error)
}

// ServerFlavor encapsula todo lo que cambia entre Java y Bedrock (D-01).
//
// Anadir Java en el hito 2 es escribir otra implementacion de esta interfaz:
// los casos de uso no cambian.
type ServerFlavor interface {
	Edition() domain.Edition
	DefaultPort() int

	// Spec construye la definicion del contenedor para esta instancia.
	Spec(inst *domain.Instance, dataDir string) ContainerSpec

	// InstallWorld extrae el mundo del archivo importado dentro de dataDir.
	InstallWorld(archivePath, dataDir, levelName string) error

	// WriteConfig genera los archivos de configuracion del servidor.
	WriteConfig(inst *domain.Instance, dataDir string, allowlist []string) error

	// ReloadAllowlist aplica la lista de permitidos sin reiniciar (H-F0-7).
	ReloadAllowlist(ctx context.Context, rt ContainerRuntime, containerID string) error

	// AvailableVersions lista las versiones instalables.
	//
	// Mojang solo publica la estable actual y la preview: NO existe un
	// historico. Por eso la interfaz ofrece estas mas una casilla para fijar
	// una concreta a mano, en vez de fingir un desplegable completo.
	AvailableVersions(ctx context.Context) ([]VersionOption, error)

	// Players consulta cuantos jugadores hay conectados. Hace falta para poder
	// avisar antes de cambiar de mapa (D-02 + D-08).
	Players(ctx context.Context, host string, port int) (online, max int, err error)
}

// VersionOption es una version que se puede instalar.
type VersionOption struct {
	Value       string
	Label       string
	Note        string
	Recommended bool
}

// WorldExtractor extrae el contenido de un archivo de mundo.
type WorldExtractor interface {
	Extract(archivePath, destDir string) error
}
