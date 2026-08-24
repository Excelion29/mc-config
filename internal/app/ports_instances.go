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
	CountByWorld(ctx context.Context, worldID int64) (int, error)
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
	// Network es la red de Docker a la que se conecta el contenedor.
	//
	// Se le pone la del panel para que este pueda preguntarle por su nombre.
	// Vacia lo deja en la red por defecto, y entonces el panel NO puede
	// alcanzarlo: Docker aisla las redes de usuario entre si.
	Network string
	// UID y GID con los que debe correr el servidor.
	//
	// Tienen que coincidir con los del PANEL: las imagenes se apropian de su
	// carpeta de datos al arrancar, y si eligen un usuario distinto el panel
	// deja de poder escribir ahi. El sintoma es callado -la lista de
	// permitidos no se actualiza y nadie puede entrar- porque el panel cree
	// que escribio.
	UID int
	GID int
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
	// Exec ejecuta una orden dentro del contenedor.
	//
	// user importa: send-command localiza el proceso del servidor recorriendo
	// /proc/*/exe, y leer eso exige ser el mismo usuario que lo lanzo o root.
	// Sin indicarlo, falla con "failed to search for bedrock server process".
	Exec(ctx context.Context, id string, cmd []string, user string) (string, error)

	// SendStdin escribe una orden en la entrada estandar del contenedor.
	//
	// Es la via fiable para hablar con el servidor: `exec send-command` busca
	// el proceso recorriendo /proc y comparando el nombre del binario, que
	// lleva la version pegada, asi que falla. stdin es donde el servidor lee
	// sus ordenes de todos modos.
	SendStdin(ctx context.Context, id, input string) error
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
	WriteConfig(inst *domain.Instance, dataDir string, allowlist []PlayerRef) error

	// ReloadAllowlist aplica la lista de permitidos sin reiniciar (H-F0-7).
	ReloadAllowlist(ctx context.Context, rt ContainerRuntime, containerID string) error
	// WritePermissions escribe quien es administrador DENTRO del juego.
	WritePermissions(dataDir string, ops []PlayerRef) error
	// ReloadPermissions los aplica sin reiniciar.
	ReloadPermissions(ctx context.Context, rt ContainerRuntime, containerID string) error

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
	Value string
	Label string
	Note  string
	// Group agrupa versiones emparentadas -"1.21", "26.2"- para que una lista
	// larga se pueda recorrer. Vacio significa suelta, sin grupo: Bedrock
	// ofrece tres opciones y agruparlas seria ruido.
	Group       string
	Recommended bool
}

// WorldExtractor extrae el contenido de un archivo de mundo.
type WorldExtractor interface {
	Extract(archivePath, destDir string) error
}

// PlayerRef identifica a un jugador ante un servidor de juego.
//
// Vive aqui y no en domain porque es la forma que piden unos archivos de
// configuracion concretos, no un concepto del problema: un jugador del dominio
// no sabe que existen allowlist.json ni whitelist.json.
//
// Las dos ediciones identifican distinto y NO son intercambiables:
//
//	Bedrock  ID = XUID (numero de Xbox Live)  Name = gamertag
//	Java     ID = UUID                        Name = nombre de Java
//
// Bedrock ademas solo usa el nombre en su allow-list, mientras que Java exige
// el UUID. Por eso viajan los dos y cada adaptador toma lo suyo.
type PlayerRef struct {
	ID   string
	Name string
}
