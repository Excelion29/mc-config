package app

import (
	"context"
	"os"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Instances resuelve los casos de uso de servidores (F3).
type Instances struct {
	repo    InstanceRepo
	maps    WorldRepo
	store   WorldStorage
	runtime ContainerRuntime
	flavors map[domain.Edition]ServerFlavor
	audit   *Audit
	clock   Clock
	log     *slog.Logger

	// dataRoot es la carpeta donde vive el /data de cada instancia.
	dataRoot string
	// host es donde se consulta el ping. En el contenedor no vale "localhost":
	// el servidor de Minecraft corre en OTRO contenedor.
	host string
	// network es la red de Docker que se asigna a cada instancia.
	network string
	// rulesOf relee las reglas del mundo al arrancar. Se inyecta despues, en
	// la raiz de composicion, porque Worlds e Instances se necesitan
	// mutuamente y encadenarlos en el constructor haria un ciclo.
	rulesOf func(context.Context, int64) (domain.Rules, error)
	// plugins instala los complementos de servidor. Puede ser nil: sin el, el
	// panel funciona igual y simplemente no ofrece el modo sin conexion.
	plugins PluginStore
	// authMode dice que modo de autenticacion esta vigente. Se inyecta despues
	// porque Access necesita Instances y al reves.
	authMode func(context.Context) domain.AuthMode
	// allowlist devuelve los gamertags de la lista maestra (D-13). Se inyecta
	// despues de construir, porque Players necesita a Instances y al reves.
	allowlist func(context.Context) ([]domain.Player, error)
	// stopTimeout es lo que se espera a un apagado limpio antes de forzar.
	// Medido en F0: ~2 s con el mundo vacio, pero un mundo jugado tarda mas.
	stopTimeout time.Duration
}

func NewInstances(
	repo InstanceRepo, maps WorldRepo, store WorldStorage,
	runtime ContainerRuntime, flavors []ServerFlavor,
	audit *Audit, clock Clock, dataRoot, host, network string, log *slog.Logger,
) *Instances {
	byEdition := make(map[domain.Edition]ServerFlavor, len(flavors))
	for _, f := range flavors {
		byEdition[f.Edition()] = f
	}

	return &Instances{
		repo: repo, maps: maps, store: store, runtime: runtime,
		flavors: byEdition, audit: audit, clock: clock,
		dataRoot: dataRoot, host: host, network: network,
		stopTimeout: 60 * time.Second, log: log,
	}
}

// SetAllowlistSource cierra el ciclo entre Players e Instances.
// SetRulesSource cierra el ciclo con Worlds, igual que SetAllowlistSource lo
// cierra con Players.
// SetPluginStore conecta el instalador de plugins.
func (i *Instances) SetPluginStore(p PluginStore) { i.plugins = p }

// SetAuthModeSource cierra el ciclo con Access.
func (i *Instances) SetAuthModeSource(f func(context.Context) domain.AuthMode) {
	i.authMode = f
}

// PluginsPara da los plugins que una edicion necesita en un modo dado.
func (i *Instances) PluginsPara(e domain.Edition, modo domain.AuthMode) []Plugin {
	flavor, ok := i.flavors[e]
	if !ok {
		return nil
	}
	proveedor, ok := flavor.(PluginProvider)
	if !ok {
		// La edicion no sabe de plugins. Bedrock esta en este caso: no los
		// admite, y por eso el modo sin conexion no le aplica.
		return nil
	}
	return proveedor.PluginsFor(modo)
}

// PluginsQueFaltan compara lo que hace falta con lo que hay en la instancia.
func (i *Instances) PluginsQueFaltan(inst *domain.Instance, requeridos []Plugin) []Plugin {
	if i.plugins == nil || len(requeridos) == 0 {
		return requeridos
	}

	puestos := map[string]bool{}
	for _, p := range i.plugins.Installed(i.dataDir(inst), requeridos) {
		puestos[p.File] = true
	}

	var faltan []Plugin
	for _, p := range requeridos {
		if !puestos[p.File] {
			faltan = append(faltan, p)
		}
	}
	return faltan
}

// InstalarPlugins descarga e instala los complementos en una instancia.
func (i *Instances) InstalarPlugins(ctx context.Context, inst *domain.Instance, lista []Plugin) error {
	if i.plugins == nil {
		return domain.ErrPluginsUnavailable
	}
	return i.plugins.Install(ctx, i.dataDir(inst), lista)
}

func (i *Instances) SetRulesSource(f func(context.Context, int64) (domain.Rules, error)) {
	i.rulesOf = f
}

// RefsPara traduce jugadores a la identidad que entiende cada edicion.
//
// Es donde vive la diferencia: Bedrock reconoce por gamertag, Java exige el
// UUID. Quien no tenga identidad valida para esa edicion se queda fuera del
// archivo, porque una entrada que el servidor no sabe interpretar no permite
// entrar a nadie y ademas confunde a quien lea el archivo.
func RefsPara(e domain.Edition, modo domain.AuthMode, jugadores []domain.Player) []PlayerRef {
	out := make([]PlayerRef, 0, len(jugadores))
	for k := range jugadores {
		p := &jugadores[k]
		switch e {
		case domain.EditionJava:
			// En Java el UUID depende del MODO, no solo de la persona: con
			// Mojang autenticando es el que el asigna, y sin conexion es uno
			// que se calcula del nombre. Quien no compro el juego no tiene el
			// primero, y ahi estaba el fallo: se le dejaba fuera de la lista
			// sin decir nada, y el rechazo al conectarse no mencionaba la
			// lista por ningun lado.
			for _, uuid := range p.IdentidadesJava(modo) {
				out = append(out, PlayerRef{ID: uuid, Name: p.JavaName})
			}
		case domain.EditionBedrock:
			if p.PuedeJugarBedrock() {
				out = append(out, PlayerRef{ID: p.XUID, Name: p.Gamertag})
			}
		}
	}
	return out
}

// modoActual lee el modo vigente, con el seguro por defecto.
func (i *Instances) modoActual(ctx context.Context) domain.AuthMode {
	if i.authMode == nil {
		return domain.AuthOnline
	}
	return i.authMode(ctx)
}

func (i *Instances) SetAllowlistSource(f func(context.Context) ([]domain.Player, error)) {
	i.allowlist = f
}

// NeedsConfirmation se devuelve cuando arrancar una instancia obliga a apagar
// otra que tiene gente jugando (D-02 + D-08).
//
// Es un tipo y no un error suelto porque quien lo reciba necesita saber a
// cuanta gente va a echar para poder avisar.
type NeedsConfirmation struct {
	Running *domain.Instance
	Players int
}

func (e *NeedsConfirmation) Error() string {
	return fmt.Sprintf("hay que detener %q antes de arrancar otra (%d jugadores conectados)",
		e.Running.Name, e.Players)
}

func (i *Instances) List(ctx context.Context, actor *domain.User) ([]domain.Instance, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return i.repo.List(ctx)
}

// All lista sin comprobar permisos. Es de uso interno: lo llama la propagacion
// de la lista maestra, que ya verifico el permiso de quien la origino.
func (i *Instances) All(ctx context.Context) ([]domain.Instance, error) {
	return i.repo.List(ctx)
}

func (i *Instances) ByID(ctx context.Context, actor *domain.User, id int64) (*domain.Instance, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return i.repo.ByID(ctx, id)
}

// specDe completa la definicion del contenedor con lo que el sabor no sabe.
//
// La red no es cosa de Bedrock ni de Java: es donde vive ESTE panel. El sabor
// describe como se levanta un servidor de su edicion; donde se enchufa lo
// decide quien lo orquesta.
func (i *Instances) specDe(ctx context.Context, flavor ServerFlavor, inst *domain.Instance, dir string) ContainerSpec {
	// El modo se pone en la instancia ANTES de pedirle la definicion al sabor.
	//
	// Aqui estaba el fallo que hizo que abrir el acceso no sirviera de nada: el
	// sabor construye ONLINE_MODE a partir de inst.Auth, y el modo se leia
	// DESPUES, sobre un campo del spec que no lee nadie. Al crear una instancia
	// inst.Auth venia vacio, asi que el contenedor salia siempre exigiendo
	// cuenta comprada -y el panel decia que el acceso estaba abierto-.
	inst.Auth = i.modoActual(ctx)

	spec := flavor.Spec(inst, dir)
	spec.Network = i.network

	// El servidor corre con el MISMO usuario que el panel.
	//
	// Las imagenes de Minecraft hacen chown de su carpeta de datos al
	// arrancar: la de Java al 1000, la de Bedrock al 65532. Si no coincide con
	// el del panel, este deja de poder escribir la configuracion y la lista de
	// permitidos, y nadie puede entrar al servidor.
	//
	// Se toma del proceso en vez de fijarlo a un numero: asi vale para
	// cualquier imagen y no depende de que dos numeros coincidan por
	// casualidad, que es justo como funcionaba antes sin que nadie lo supiera.
	spec.UID, spec.GID = os.Getuid(), os.Getgid()

	return spec
}

// dataDir es el /data de una instancia en el host.
func (i *Instances) dataDir(inst *domain.Instance) string {
	return filepath.Join(i.dataRoot, inst.Slug)
}

// ApplyAllowlist reescribe la lista de permitidos y la recarga en caliente.
// Lo usara F4 con la lista maestra (D-13).
func (i *Instances) ApplyAllowlist(ctx context.Context, inst *domain.Instance, jugadores []domain.Player) error {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return domain.ErrEditionMismatch
	}

	// WriteConfig reescribe server.properties entero, asi que necesita el modo
	// vigente. Sin esto, propagar la lista de permitidos volveria a poner
	// online-mode=true y desactivaria a los no premium sin que nada lo diga.
	inst.Auth = i.modoActual(ctx)
	if err := flavor.WriteConfig(inst, i.dataDir(inst), RefsPara(inst.Edition, inst.Auth, jugadores)); err != nil {
		return err
	}
	if inst.State == domain.StateRunning {
		return flavor.ReloadAllowlist(ctx, i.runtime, inst.ContainerID)
	}
	return nil
}

// ensureContainer recrea el contenedor si desaparecio.
func (i *Instances) ensureContainer(ctx context.Context, inst *domain.Instance) error {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return domain.ErrEditionMismatch
	}

	spec := i.specDe(ctx, flavor, inst, i.dataDir(inst))
	huella := spec.Huella()

	if inst.ContainerID != "" {
		if st, err := i.runtime.Status(ctx, inst.ContainerID); err == nil && st.Exists {
			// Existe, pero puede estar hecho con una definicion vieja. El
			// entorno, los puertos y los limites se fijan al CREARLO: no hay
			// forma de cambiarlos en caliente ni reiniciando.
			//
			// Aqui se destapo con el modo de autenticacion. Abrir el acceso a
			// cuentas no premium no surtia efecto en un servidor ya creado,
			// porque ONLINE_MODE seguia siendo el del dia que se creo. Y el
			// servidor arrancaba perfectamente, asi que no habia nada que
			// mirar: el sintoma era un amigo que no entra.
			if inst.SpecHash == huella {
				return nil
			}

			i.log.Info("la definicion del contenedor cambio; se rehace",
				"instancia", inst.Name)
			if err := i.runtime.Remove(ctx, inst.ContainerID); err != nil {
				return fmt.Errorf("retirando el contenedor viejo: %w", err)
			}
			inst.ContainerID = ""
		}
	}

	id, err := i.runtime.Create(ctx, spec)
	if err != nil {
		return err
	}
	inst.ContainerID = id
	inst.SpecHash = huella
	return i.repo.SetContainer(ctx, inst.ID, id, huella)
}

func (i *Instances) playersOf(ctx context.Context, inst *domain.Instance) (int, int, error) {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return 0, 0, domain.ErrEditionMismatch
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Compartiendo red se pregunta al contenedor por su nombre y por el puerto
	// INTERNO: no hay NAT de por medio, asi que el puerto publicado en el host
	// no pinta nada aqui.
	host, puerto := i.host, inst.Port
	if i.network != "" {
		host, puerto = "mcvps-"+inst.Slug, flavor.DefaultPort()
	}

	online, max, err := flavor.Players(ctx, host, puerto)
	if err != nil {
		// Este fallo NO era visible en ningun sitio, y es el que decide si una
		// instancia pasa de "arrancando" a "activa". Sin esta linea el sintoma
		// es una pantalla clavada en "arrancando" y cero pistas: ni en el log
		// del panel ni en el del servidor, porque el servidor esta
		// perfectamente y quien no llega es el panel.
		//
		// Se registra la direccion consultada a proposito: casi siempre el
		// problema es esa, no el servidor.
		i.log.Warn("el servidor no responde al ping; sigue en arrancando",
			"instancia", inst.Name, "host", host, "puerto", puerto, "error", err)
	}
	return online, max, err
}

// fail marca la instancia como fallida y deja el motivo en el log del proceso.
func (i *Instances) fail(ctx context.Context, inst *domain.Instance, cause error) {
	i.log.Error("la instancia quedo en estado de fallo",
		"instancia", inst.Name, "error", cause)
	if err := i.repo.SetState(ctx, inst.ID, domain.StateFailed); err != nil {
		i.log.Error("ademas fallo al guardar el estado", "error", err)
	}
}

// Running devuelve la instancia encendida, o nil si no hay ninguna.
//
// Es interno del panel y no lleva actor: lo usa el vigilante de conexiones,
// que no actua en nombre de nadie.
func (i *Instances) Running(ctx context.Context) (*domain.Instance, error) {
	list, err := i.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for k := range list {
		if list[k].State == domain.StateRunning {
			return &list[k], nil
		}
	}
	return nil, nil
}

// RawLogs lee el log de una instancia sin comprobar permisos.
//
// Existe aparte de Logs porque los que llaman son distintos: Logs atiende a una
// persona y por tanto exige permiso; esto lo usa el vigilante, que es el propio
// panel mirando su servidor. Pedirle un actor obligaria a inventarse un usuario
// falso, y un usuario falso con permisos es justo lo que no queremos tener.
func (i *Instances) RawLogs(ctx context.Context, inst *domain.Instance, lines int) (string, error) {
	if inst == nil || inst.ContainerID == "" {
		return "", nil
	}
	return i.runtime.Logs(ctx, inst.ContainerID, lines)
}

// ApplyOps reescribe permissions.json de TODAS las instancias y lo recarga en
// la que este encendida.
//
// Igual que la allow-list, los permissions.json son derivados: la verdad esta
// en la base. No devuelve error a proposito -el alta ya esta guardada- y las
// instancias paradas lo recogeran al arrancar.
func (i *Instances) ApplyOps(ctx context.Context, jugadores []domain.Player) {
	ops := OpsFrom(jugadores)

	list, err := i.repo.List(ctx)
	if err != nil {
		i.log.Error("no se pudieron listar las instancias", "error", err)
		return
	}

	for k := range list {
		inst := &list[k]
		flavor, ok := i.flavors[inst.Edition]
		if !ok {
			continue
		}
		if err := flavor.WritePermissions(i.dataDir(inst), ops); err != nil {
			i.log.Warn("no se pudo escribir permissions.json",
				"instancia", inst.Name, "error", err)
			continue
		}
		if inst.State == domain.StateRunning {
			if err := flavor.ReloadPermissions(ctx, i.runtime, inst.ContainerID); err != nil {
				i.log.Warn("no se pudo recargar permissions.json",
					"instancia", inst.Name, "error", err)
			}
		}
		i.log.Info("operadores aplicados", "instancia", inst.Name, "ops", len(ops))
	}
}
