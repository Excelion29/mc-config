package app

import (
	"context"
	"log/slog"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Players resuelve los casos de uso de la lista maestra (F4).
//
// La idea de D-13: la verdad vive en la base, y los allowlist.json de cada
// instancia son DERIVADOS. Dar de alta a un amigo una vez lo habilita en todos
// los mapas, presentes y futuros, sin reiniciar nada.
type Players struct {
	repo      PlayerRepo
	instances *Instances
	audit     *Audit
	clock     Clock
	log       *slog.Logger
}

func NewPlayers(repo PlayerRepo, instances *Instances, audit *Audit, clock Clock, log *slog.Logger) *Players {
	return &Players{repo: repo, instances: instances, audit: audit, clock: clock, log: log}
}

func (p *Players) List(ctx context.Context, actor *domain.User) ([]domain.Player, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return nil, domain.ErrForbidden
	}
	return p.repo.List(ctx)
}

// ActiveGamertags lo usa Instances al crear una instancia nueva, para que nazca
// ya con la lista puesta en vez de vacia.
func (p *Players) ActiveGamertags(ctx context.Context) ([]string, error) {
	return p.repo.ActiveGamertags(ctx)
}

func (p *Players) Add(ctx context.Context, actor *domain.User, gamertag, note string, isOp bool, ip string) (*domain.Player, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return nil, domain.ErrForbidden
	}

	gamertag = domain.NormalizeGamertag(gamertag)
	if gamertag == "" {
		return nil, domain.ErrEmptyGamertag
	}

	player := &domain.Player{
		Gamertag:  gamertag,
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

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPlayerAdded, gamertag, ip)
	p.propagate(ctx)
	return player, nil
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
	names, err := p.repo.ActiveGamertags(ctx)
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
		if err := p.instances.ApplyAllowlist(ctx, inst, names); err != nil {
			p.log.Warn("no se pudo aplicar la lista a una instancia",
				"instancia", inst.Name, "error", err)
			continue
		}
		p.log.Info("lista de permitidos aplicada",
			"instancia", inst.Name, "jugadores", len(names))
	}
}
