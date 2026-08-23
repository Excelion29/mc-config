package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Instances resuelve los casos de uso de servidores (F3).
type Instances struct {
	repo    InstanceRepo
	maps    MapRepo
	store   MapStorage
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
	// stopTimeout es lo que se espera a un apagado limpio antes de forzar.
	// Medido en F0: ~2 s con el mundo vacio, pero un mundo jugado tarda mas.
	stopTimeout time.Duration
}

func NewInstances(
	repo InstanceRepo, maps MapRepo, store MapStorage,
	runtime ContainerRuntime, flavors []ServerFlavor,
	audit *Audit, clock Clock, dataRoot, host string, log *slog.Logger,
) *Instances {
	byEdition := make(map[domain.Edition]ServerFlavor, len(flavors))
	for _, f := range flavors {
		byEdition[f.Edition()] = f
	}

	return &Instances{
		repo: repo, maps: maps, store: store, runtime: runtime,
		flavors: byEdition, audit: audit, clock: clock,
		dataRoot: dataRoot, host: host, stopTimeout: 60 * time.Second, log: log,
	}
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

func (i *Instances) ByID(ctx context.Context, actor *domain.User, id int64) (*domain.Instance, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return i.repo.ByID(ctx, id)
}

// dataDir es el /data de una instancia en el host.
func (i *Instances) dataDir(inst *domain.Instance) string {
	return filepath.Join(i.dataRoot, inst.Slug)
}

// Create prepara un servidor a partir de un mapa de la biblioteca.
//
// Instala el mundo, escribe la configuracion y precrea el contenedor, pero NO
// lo arranca: por D-02 solo puede haber uno encendido, y decidir cual es una
// accion aparte.
func (i *Instances) Create(ctx context.Context, actor *domain.User, name string, mapID int64, version string, allowlist []string, ip string) (*domain.Instance, error) {
	if !actor.Can(domain.PermInstanceCreate) {
		return nil, domain.ErrForbidden
	}

	name = strings.TrimSpace(name)
	mp, err := i.maps.ByID(ctx, mapID)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = mp.Name
	}

	slug := domain.Slugify(name)
	if slug == "" {
		return nil, domain.ErrEmptyInstanceName
	}
	if _, err := i.repo.BySlug(ctx, slug); err == nil {
		return nil, domain.ErrDuplicateInstance
	} else if !errors.Is(err, domain.ErrInstanceNotFound) {
		return nil, err
	}

	// D-01: un .mcworld no va a un servidor Java, y al reves tampoco. Se
	// comprueba aqui y no al arrancar, para no dejar una instancia creada que
	// nunca podra funcionar.
	flavor, ok := i.flavors[mp.Edition]
	if !ok {
		return nil, fmt.Errorf("%w: no hay soporte para %s", domain.ErrEditionMismatch, mp.Edition.Label())
	}

	// La version del MAPA y la del SERVIDOR no son lo mismo, y confundirlas
	// rompe el arranque.
	//
	// level.dat guarda con que version se abrio el mundo por ultima vez
	// (p.ej. 1.21.21.3). Eso es un REQUISITO -"necesita 1.20.80 o superior"-,
	// no un numero de build descargable: Mojang no publica un servidor con ese
	// numero y la descarga devuelve 404. Confirmado en F0 y otra vez aqui.
	//
	// Por eso por defecto se instala LATEST, que es lo que funciono en F0.
	version = strings.TrimSpace(version)
	if version == "" {
		version = "LATEST"
	}

	inst := &domain.Instance{
		Name:      name,
		Slug:      slug,
		Edition:   mp.Edition,
		Version:   version,
		MapID:     mp.ID,
		MapName:   mp.Name,
		LevelName: slug,
		Port:      flavor.DefaultPort(),
		State:     domain.StateStopped,
		MemoryMB:  domain.DefaultMemoryMB,
		CPUs:      domain.DefaultCPUs,
		CreatedAt: i.clock(),
	}

	dir := i.dataDir(inst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creando %s: %w", dir, err)
	}

	// Si algo falla a partir de aqui, no debe quedar media instancia en disco.
	limpiar := func() { os.RemoveAll(dir) }

	if err := flavor.InstallWorld(i.store.ArchivePath(mp.SHA256), dir, inst.LevelName); err != nil {
		limpiar()
		return nil, err
	}
	if err := flavor.WriteConfig(inst, dir, allowlist); err != nil {
		limpiar()
		return nil, err
	}

	containerID, err := i.runtime.Create(ctx, flavor.Spec(inst, dir))
	if err != nil {
		limpiar()
		return nil, err
	}
	inst.ContainerID = containerID

	id, err := i.repo.Create(ctx, inst)
	if err != nil {
		i.runtime.Remove(ctx, containerID)
		limpiar()
		return nil, err
	}
	inst.ID = id

	i.audit.Record(ctx, actor, actor.Email, domain.ActionInstanceCreated,
		fmt.Sprintf("%s (%s %s) desde el mapa %s", inst.Name, inst.Edition.Label(), inst.Version, mp.Name), ip)
	return inst, nil
}

// Versions lista las versiones instalables de una edicion.
func (i *Instances) Versions(ctx context.Context, actor *domain.User, edition domain.Edition) ([]VersionOption, error) {
	if !actor.Can(domain.PermInstanceCreate) {
		return nil, domain.ErrForbidden
	}
	flavor, ok := i.flavors[edition]
	if !ok {
		return nil, domain.ErrEditionMismatch
	}
	return flavor.AvailableVersions(ctx)
}

// SwitchPreview responde a "si arranco esta, a quien echo".
//
// Se separa de Start porque la confirmacion viaja por la URL tras una
// redireccion (POST/Redirect/GET) y hay que poder recomponerla en el GET.
func (i *Instances) SwitchPreview(ctx context.Context, actor *domain.User, targetID int64) (*domain.Instance, int, error) {
	if !actor.Can(domain.PermServerOperate) {
		return nil, 0, domain.ErrForbidden
	}

	running, err := i.repo.Running(ctx)
	if errors.Is(err, domain.ErrInstanceNotFound) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if running.ID == targetID {
		return nil, 0, nil
	}

	online, _, _ := i.playersOf(ctx, running)
	return running, online, nil
}

// Start enciende una instancia.
//
// Por D-02 solo puede haber una encendida: si hay otra, se detiene primero. Y
// como eso desconecta a quien este jugando, no se hace sin confirmacion (D-08).
func (i *Instances) Start(ctx context.Context, actor *domain.User, id int64, confirmed bool, ip string) error {
	if !actor.Can(domain.PermServerOperate) {
		return domain.ErrForbidden
	}

	inst, err := i.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if inst.State.Busy() {
		return domain.ErrInstanceBusy
	}
	if inst.State == domain.StateRunning {
		return nil
	}

	if running, err := i.repo.Running(ctx); err == nil && running.ID != inst.ID {
		if !confirmed {
			online, _, _ := i.playersOf(ctx, running)
			return &NeedsConfirmation{Running: running, Players: online}
		}
		if err := i.stop(ctx, actor, running, ip, true); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, domain.ErrInstanceNotFound) {
		return err
	}

	if err := i.repo.SetState(ctx, inst.ID, domain.StateStarting); err != nil {
		return err
	}

	if err := i.ensureContainer(ctx, inst); err != nil {
		i.fail(ctx, inst, err)
		return err
	}
	if err := i.runtime.Start(ctx, inst.ContainerID); err != nil {
		i.fail(ctx, inst, err)
		return err
	}

	// Solo se comprueba que no muera de inmediato: version inexistente, mundo
	// ilegible o puerto ocupado matan el contenedor en segundos.
	//
	// NO se espera aqui a que el servidor este en pie. La primera vez la imagen
	// descarga el binario de Mojang y eso tarda minutos: bloquear la peticion
	// tanto rato no es opcion. La instancia queda en "arrancando" y es Status
	// quien la promueve cuando el servidor responde de verdad.
	if err := i.diedImmediately(ctx, inst); err != nil {
		i.fail(ctx, inst, err)
		return err
	}

	i.repo.MarkStarted(ctx, inst.ID, i.clock())

	i.audit.Record(ctx, actor, actor.Email, domain.ActionInstanceStarted, inst.Name, ip)
	return nil
}

// diedImmediately detecta un arranque que se cae al momento.
//
// Devuelve el error CON las ultimas lineas del log: sin eso, la pantalla solo
// diria "con fallo" y habria que ir a buscar el motivo a mano.
func (i *Instances) diedImmediately(ctx context.Context, inst *domain.Instance) error {
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)

		st, err := i.runtime.Status(ctx, inst.ContainerID)
		if err != nil {
			return err
		}
		if st.Exists && !st.Running {
			logs, _ := i.runtime.Logs(ctx, inst.ContainerID, 15)
			return fmt.Errorf("el servidor se detuvo al arrancar (codigo %d): %s",
				st.ExitCode, ultimasLineas(logs, 5))
		}
	}
	return nil
}

// ultimasLineas recorta el log a lo util para un mensaje en pantalla.
func ultimasLineas(logs string, n int) string {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	var utiles []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		// Las lineas de depuracion del arrancador solo hacen ruido.
		if l == "" || strings.Contains(l, `level=debug`) {
			continue
		}
		utiles = append(utiles, l)
	}
	if len(utiles) > n {
		utiles = utiles[len(utiles)-n:]
	}
	return strings.Join(utiles, "\n")
}

// Stop detiene una instancia con apagado limpio.
func (i *Instances) Stop(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermServerOperate) {
		return domain.ErrForbidden
	}

	inst, err := i.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if inst.State.Busy() {
		return domain.ErrInstanceBusy
	}
	return i.stop(ctx, actor, inst, ip, false)
}

// stop hace el trabajo. switching indica que se para para arrancar otra, y solo
// cambia el detalle que queda registrado.
func (i *Instances) stop(ctx context.Context, actor *domain.User, inst *domain.Instance, ip string, switching bool) error {
	if inst.ContainerID == "" {
		return i.repo.SetState(ctx, inst.ID, domain.StateStopped)
	}

	if err := i.repo.SetState(ctx, inst.ID, domain.StateStopping); err != nil {
		return err
	}

	// StopAndWait espera a que el contenedor llegue a "exited". Nunca se fuerza
	// aqui: un corte a lo bruto corrompe el mundo (H-F0-6).
	if err := i.runtime.StopAndWait(ctx, inst.ContainerID, i.stopTimeout); err != nil {
		i.fail(ctx, inst, err)
		return fmt.Errorf("no se pudo detener %q limpiamente: %w", inst.Name, err)
	}

	if err := i.repo.SetState(ctx, inst.ID, domain.StateStopped); err != nil {
		return err
	}

	detalle := inst.Name
	if switching {
		detalle += " (para arrancar otra)"
	}
	i.audit.Record(ctx, actor, actor.Email, domain.ActionInstanceStopped, detalle, ip)
	return nil
}

// Delete borra la instancia, su contenedor y su mundo.
func (i *Instances) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermInstanceDelete) {
		return domain.ErrForbidden
	}

	inst, err := i.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	// Borrar algo encendido dejaria el contenedor huerfano y el mundo a medio
	// guardar. Se exige pararlo antes, conscientemente.
	if inst.State != domain.StateStopped && inst.State != domain.StateFailed {
		return domain.ErrInstanceRunning
	}

	if inst.ContainerID != "" {
		if err := i.runtime.Remove(ctx, inst.ContainerID); err != nil {
			return err
		}
	}
	if err := i.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Los archivos al final: al reves, un fallo dejaria una fila apuntando a un
	// mundo que ya no existe.
	if err := os.RemoveAll(i.dataDir(inst)); err != nil {
		i.log.Warn("la instancia se borro pero quedaron archivos",
			"slug", inst.Slug, "error", err)
	}

	i.audit.Record(ctx, actor, actor.Email, domain.ActionInstanceDeleted, inst.Name, ip)
	return nil
}

// Status devuelve el estado real del contenedor y los jugadores conectados.
//
// Se consulta a Docker y no solo a la base: el contenedor pudo caerse por su
// cuenta y la fila seguiria diciendo "running".
func (i *Instances) Status(ctx context.Context, actor *domain.User, id int64) (*domain.Instance, int, int, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, 0, 0, domain.ErrForbidden
	}

	inst, err := i.repo.ByID(ctx, id)
	if err != nil {
		return nil, 0, 0, err
	}

	if inst.ContainerID != "" {
		st, err := i.runtime.Status(ctx, inst.ContainerID)
		if err == nil {
			switch {
			case inst.State == domain.StateStarting:
				// La senal fiable de que un servidor esta en pie NO es que el
				// contenedor viva: puede estar descargando el binario durante
				// minutos. Es que responda al ping.
				switch {
				case st.Exists && !st.Running:
					i.repo.SetState(ctx, inst.ID, domain.StateFailed)
					inst.State = domain.StateFailed
				case st.Running:
					if online, max, err := i.playersOf(ctx, inst); err == nil {
						i.repo.SetState(ctx, inst.ID, domain.StateRunning)
						inst.State = domain.StateRunning
						return inst, online, max, nil
					}
				}

			case inst.State == domain.StateRunning:
				// Se cayo solo. Se marca como fallo y no como detenida: alguien
				// tiene que mirar los logs antes de volver a arrancarla.
				if !st.Exists || !st.Running {
					i.repo.SetState(ctx, inst.ID, domain.StateFailed)
					inst.State = domain.StateFailed
				}
			}
		}
	}

	if inst.State != domain.StateRunning {
		return inst, 0, 0, nil
	}

	online, max, _ := i.playersOf(ctx, inst)
	return inst, online, max, nil
}

// Logs devuelve las ultimas lineas del servidor.
func (i *Instances) Logs(ctx context.Context, actor *domain.User, id int64, lines int) (string, error) {
	if !actor.Can(domain.PermServerView) {
		return "", domain.ErrForbidden
	}

	inst, err := i.repo.ByID(ctx, id)
	if err != nil {
		return "", err
	}
	return i.runtime.Logs(ctx, inst.ContainerID, lines)
}

// ApplyAllowlist reescribe la lista de permitidos y la recarga en caliente.
// Lo usara F4 con la lista maestra (D-13).
func (i *Instances) ApplyAllowlist(ctx context.Context, inst *domain.Instance, names []string) error {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return domain.ErrEditionMismatch
	}
	if err := flavor.WriteConfig(inst, i.dataDir(inst), names); err != nil {
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

	if inst.ContainerID != "" {
		if st, err := i.runtime.Status(ctx, inst.ContainerID); err == nil && st.Exists {
			return nil
		}
	}

	id, err := i.runtime.Create(ctx, flavor.Spec(inst, i.dataDir(inst)))
	if err != nil {
		return err
	}
	inst.ContainerID = id
	return i.repo.SetContainer(ctx, inst.ID, id)
}

func (i *Instances) playersOf(ctx context.Context, inst *domain.Instance) (int, int, error) {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return 0, 0, domain.ErrEditionMismatch
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return flavor.Players(ctx, i.host, inst.Port)
}

// fail marca la instancia como fallida y deja el motivo en el log del proceso.
func (i *Instances) fail(ctx context.Context, inst *domain.Instance, cause error) {
	i.log.Error("la instancia quedo en estado de fallo",
		"instancia", inst.Name, "error", cause)
	if err := i.repo.SetState(ctx, inst.ID, domain.StateFailed); err != nil {
		i.log.Error("ademas fallo al guardar el estado", "error", err)
	}
}
