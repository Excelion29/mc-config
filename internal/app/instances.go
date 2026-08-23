package app

import (
	"context"
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
	// allowlist devuelve los gamertags de la lista maestra (D-13). Se inyecta
	// despues de construir, porque Players necesita a Instances y al reves.
	allowlist func(context.Context) ([]string, error)
	// stopTimeout es lo que se espera a un apagado limpio antes de forzar.
	// Medido en F0: ~2 s con el mundo vacio, pero un mundo jugado tarda mas.
	stopTimeout time.Duration
}

func NewInstances(
	repo InstanceRepo, maps WorldRepo, store WorldStorage,
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

// SetAllowlistSource cierra el ciclo entre Players e Instances.
func (i *Instances) SetAllowlistSource(f func(context.Context) ([]string, error)) {
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

// dataDir es el /data de una instancia en el host.
func (i *Instances) dataDir(inst *domain.Instance) string {
	return filepath.Join(i.dataRoot, inst.Slug)
}

// ApplyAllowlist reescribe la lista de permitidos y la recarga en caliente.
// Lo usara F4 con la lista maestra (D-13).
func (i *Instances) ApplyAllowlist(ctx context.Context, inst *domain.Instance, names []string) error {
	flavor, ok := i.flavors[inst.Edition]
	if !ok {
		return domain.ErrEditionMismatch
	}
	if err := flavor.WriteConfig(inst, i.dataDir(inst), RefsFrom(names)); err != nil {
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
