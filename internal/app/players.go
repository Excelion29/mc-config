package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Players resuelve los casos de uso de la lista maestra (F4).
//
// La idea de D-13: la verdad vive en la base, y los allowlist.json de cada
// instancia son DERIVADOS. Dar de alta a un amigo una vez lo habilita en todos
// los mapas, presentes y futuros, sin reiniciar nada.
// JavaIdentity traduce un nombre de Java a su UUID.
//
// Es un puerto porque hablar con Mojang es un detalle de infraestructura: los
// casos de uso solo saben que un nombre de Java se puede convertir en algo que
// el servidor entiende.
type JavaIdentity interface {
	ResolveJavaUUID(ctx context.Context, nombre string) (string, error)
}

type Players struct {
	repo      PlayerRepo
	java      JavaIdentity
	instances *Instances
	// authMode dice si hace falta cuenta comprada. Se inyecta despues, como en
	// Instances, porque Access necesita a los dos.
	authMode func(context.Context) domain.AuthMode
	audit    *Audit
	clock    Clock
	log      *slog.Logger
}

func NewPlayers(repo PlayerRepo, java JavaIdentity, instances *Instances, audit *Audit, clock Clock, log *slog.Logger) *Players {
	return &Players{repo: repo, java: java, instances: instances, audit: audit, clock: clock, log: log}
}

// SetAuthModeSource cierra el ciclo con Access.
func (p *Players) SetAuthModeSource(f func(context.Context) domain.AuthMode) {
	p.authMode = f
}

func (p *Players) modoActual(ctx context.Context) domain.AuthMode {
	if p.authMode == nil {
		return domain.AuthOnline
	}
	return p.authMode(ctx)
}

func (p *Players) List(ctx context.Context, actor *domain.User) ([]domain.Player, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return nil, domain.ErrForbidden
	}
	return p.repo.List(ctx)
}

// ByID devuelve un jugador suelto. Lo usa la web para repintar una fila
// despues de cambiarla, releyendo lo que quedo guardado en vez de fiarse de lo
// que venia en la peticion.
func (p *Players) ByID(ctx context.Context, actor *domain.User, id int64) (*domain.Player, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return nil, domain.ErrForbidden
	}
	return p.repo.ByID(ctx, id)
}

// Permitidos lo usa Instances para generar la lista de cada servidor.
//
// Devuelve jugadores ENTEROS y no nombres: cada edicion identifica distinto y
// quien escribe el archivo necesita poder elegir el campo. Con solo nombres,
// Java no podria escribir su whitelist, que va por UUID.
func (p *Players) Permitidos(ctx context.Context) ([]domain.Player, error) {
	return p.repo.Permitidos(ctx)
}

func (p *Players) Add(ctx context.Context, actor *domain.User, gamertag, javaName, note string, isOp bool, ip string) (*domain.Player, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return nil, domain.ErrForbidden
	}

	gamertag = domain.NormalizeGamertag(gamertag)
	javaName = domain.NormalizeGamertag(javaName)

	// Hace falta al menos UNA identidad. Alguien sin gamertag ni nombre de
	// Java no puede entrar a ningun sitio, y darlo de alta seria guardar una
	// fila que no significa nada.
	if gamertag == "" && javaName == "" {
		return nil, domain.ErrEmptyGamertag
	}

	// El UUID se resuelve AHORA, no cuando la persona entre. En Java se puede
	// preguntar a Mojang de antemano (H-J-8), y hacerlo aqui tiene una ventaja
	// que no es solo tecnica: si el nombre esta mal escrito, se sabe en este
	// momento y no dentro de una semana cuando alguien no pueda entrar.
	javaUUID := ""
	if javaName != "" && p.java != nil {
		uuid, err := p.java.ResolveJavaUUID(ctx, javaName)
		switch {
		case err == nil:
			javaUUID = uuid
		case errors.Is(err, domain.ErrJavaNameNotFound):
			// Que Mojang no lo conozca solo es un error si hace falta tener el
			// juego comprado. Con el acceso abierto es lo NORMAL -es
			// exactamente el caso que queriamos cubrir- y rechazarlo dejaba
			// fuera del panel a la gente para la que se hizo F6, con un
			// "jugador invalido" que no explicaba nada.
			//
			// Se guarda sin UUID a proposito: el de sin conexion se calcula del
			// nombre al escribir la lista. Guardarlo aqui seria fijar una
			// identidad que deja de valer en cuanto se cierre el acceso.
			if !p.modoActual(ctx).SinConexion() {
				return nil, err
			}
			p.log.Info("nombre de Java desconocido para Mojang; se acepta porque el acceso esta abierto",
				"nombre", javaName)
		default:
			// Mojang caido no debe impedir dar de alta a alguien: se guarda
			// sin UUID y no podra entrar a Java hasta que se reintente. La
			// pantalla lo dice, no se finge que esta listo.
			p.log.Warn("no se pudo resolver el UUID de Java; el jugador queda sin acceso a Java",
				"nombre", javaName, "error", err)
		}
	}

	player := &domain.Player{
		Gamertag:  gamertag,
		JavaName:  javaName,
		JavaUUID:  javaUUID,
		Note:      note,
		IsOp:      isOp,
		Active:    true,
		CreatedAt: p.clock(),
	}

	id, err := p.repo.Create(ctx, player)
	if err != nil {
		return nil, err
	}
	player.ID = id

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPlayerAdded, player.Etiqueta(), ip)
	p.propagate(ctx)
	return player, nil
}

// Update corrige las identidades de alguien que ya esta dado de alta.
//
// Existe porque sin esto, anadir el nombre de Java a quien se dio de alta solo
// con su gamertag obligaba a BORRARLO y volver a crearlo. Y borrarlo no es
// gratis: se lleva por delante su nota, cuando se le dio de alta y si era
// operador.
//
// No toca ni el bloqueo ni el rango de operador. Esos tienen su propio boton y
// su propia linea en el registro; reescribirlos de paso podria desbloquear a
// alguien sin querer al corregirle una letra.
func (p *Players) Update(ctx context.Context, actor *domain.User, id int64,
	gamertag, javaName, note, ip string) error {

	if !actor.Can(domain.PermPlayerManage) {
		return domain.ErrForbidden
	}

	jugador, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}

	gamertag = domain.NormalizeGamertag(gamertag)
	javaName = domain.NormalizeGamertag(javaName)
	if gamertag == "" && javaName == "" {
		return domain.ErrEmptyGamertag
	}

	// El UUID se vuelve a pedir solo si el nombre de Java CAMBIO. Es lo unico
	// que obliga a salir a la red, y hacerlo por corregir una nota seria
	// preguntarle a Mojang por gusto.
	if javaName != jugador.JavaName {
		jugador.JavaUUID = ""
		if javaName != "" && p.java != nil {
			uuid, err := p.java.ResolveJavaUUID(ctx, javaName)
			switch {
			case err == nil:
				jugador.JavaUUID = uuid
			case errors.Is(err, domain.ErrJavaNameNotFound):
				// Con el acceso abierto, que Mojang no lo conozca es lo normal:
				// el UUID de un no premium se calcula del nombre al escribir la
				// lista (D-17).
				if !p.modoActual(ctx).SinConexion() {
					return err
				}
			default:
				p.log.Warn("no se pudo resolver el UUID de Java al editar",
					"nombre", javaName, "error", err)
			}
		}
	}

	jugador.Gamertag, jugador.JavaName, jugador.Note = gamertag, javaName, strings.TrimSpace(note)
	if err := p.repo.Update(ctx, jugador); err != nil {
		return err
	}

	// La lista de permitidos se rehace: si acaba de ganar identidad en Java, ya
	// puede entrar sin esperar a un reinicio.
	p.propagate(ctx)

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPlayerUpdated, jugador.Etiqueta(), ip)
	return nil
}

func (p *Players) SetActive(ctx context.Context, actor *domain.User, id int64, active bool, ip string) error {
	if !actor.Can(domain.PermPlayerManage) {
		return domain.ErrForbidden
	}

	player, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.repo.SetActive(ctx, id, active); err != nil {
		return err
	}

	action := domain.ActionPlayerDisabled
	if active {
		action = domain.ActionPlayerEnabled
	}
	p.audit.Record(ctx, actor, actor.Email, action, player.Gamertag, ip)
	p.propagate(ctx)
	return nil
}

func (p *Players) SetOp(ctx context.Context, actor *domain.User, id int64, isOp bool, ip string) error {
	if !actor.Can(domain.PermPlayerManage) {
		return domain.ErrForbidden
	}

	player, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.repo.SetOp(ctx, id, isOp); err != nil {
		return err
	}

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPlayerOp, player.Gamertag, ip)
	p.propagate(ctx)
	return nil
}

func (p *Players) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermPlayerManage) {
		return domain.ErrForbidden
	}

	player, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := p.repo.Delete(ctx, id); err != nil {
		return err
	}

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPlayerRemoved, player.Gamertag, ip)
	p.propagate(ctx)
	return nil
}

// propagate reescribe el allowlist.json de TODAS las instancias y lo recarga en
// caliente en la que este encendida.
//
// No devuelve error a proposito: el alta ya esta guardada y es la verdad. Si una
// instancia parada no se puede reescribir ahora, tomara la lista al arrancar,
// porque se regenera tambien entonces. Fallar aqui obligaria a deshacer un alta
// correcta por un problema de otra cosa.
func (p *Players) propagate(ctx context.Context) {
	jugadores, err := p.repo.Permitidos(ctx)
	if err != nil {
		p.log.Error("no se pudo leer la lista maestra para propagarla", "error", err)
		return
	}

	list, err := p.instances.All(ctx)
	if err != nil {
		p.log.Error("no se pudieron listar las instancias", "error", err)
		return
	}

	for i := range list {
		inst := &list[i]
		if err := p.instances.ApplyAllowlist(ctx, inst, jugadores); err != nil {
			p.log.Warn("no se pudo aplicar la lista a una instancia",
				"instancia", inst.Name, "error", err)
			continue
		}
		p.log.Info("lista de permitidos aplicada",
			"instancia", inst.Name, "jugadores", len(jugadores))
	}
}
