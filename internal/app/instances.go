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
	// allowlist devuelve los gamertags de la lista maestra (D-13). Se inyecta
	// despues de construir, porque Players necesita a Instances y al reves.
	allowlist func(context.Context) ([]string, error)
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
