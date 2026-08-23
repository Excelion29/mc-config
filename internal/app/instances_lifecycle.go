package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Este archivo cubre el CICLO DE VIDA de un servidor ya preparado: arrancar,
// parar, consultar estado y borrar.
//
// Va aparte de la creacion porque aqui vive la regla de que solo puede haber
// uno encendido a la vez, y esa regla es la que hace que arrancar sea la
// operacion mas delicada del panel: puede tener que apagar otro antes.

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

	// La lista de permitidos se regenera en cada arranque desde la maestra:
	// una instancia parada pudo perderse altas o bajas mientras no corria.
	if i.allowlist != nil {
		if names, err := i.allowlist(ctx); err == nil {
			if flavor, ok := i.flavors[inst.Edition]; ok {
				if err := flavor.WriteConfig(inst, i.dataDir(inst), names); err != nil {
					i.log.Warn("no se pudo escribir la lista de permitidos",
						"instancia", inst.Name, "error", err)
				}
			}
		}
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
