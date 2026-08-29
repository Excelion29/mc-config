package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// PackRepo guarda la biblioteca de paquetes y a que mundo va cada uno.
type PackRepo interface {
	List(ctx context.Context) ([]domain.Pack, error)
	ByID(ctx context.Context, id int64) (*domain.Pack, error)
	Create(ctx context.Context, p *domain.Pack) (int64, error)
	Update(ctx context.Context, p *domain.Pack) error
	Delete(ctx context.Context, id int64) error

	DeMundo(ctx context.Context, worldID int64) ([]domain.PackAsignado, error)
	Asignar(ctx context.Context, worldID int64, ids []int64, activo int64) error
	ActivoDe(ctx context.Context, worldID int64) (domain.PackRef, error)
	SetRequired(ctx context.Context, worldID int64, requerido bool) error
}

// PackHasher calcula el SHA-1 de un paquete a partir de su enlace.
//
// Es un puerto porque descargar es infraestructura. Y existe porque ese hash lo
// necesita Minecraft y NADIE lo tiene a mano: no viene en la pagina de descarga
// ni lo publica el autor. Pedirselo a quien anade el paquete seria pedirle algo
// que no puede dar.
type PackHasher interface {
	// SHA1 descarga el archivo, lo hashea y TIRA los bytes.
	//
	// No se guarda nada: el panel no aloja paquetes, solo enlaces. Se paga una
	// descarga una vez, al anadirlo, para ahorrarsela a cada jugador en cada
	// conexion.
	SHA1(ctx context.Context, url string) (string, error)
}

// Packs es la biblioteca de paquetes de texturas.
type Packs struct {
	repo   PackRepo
	hasher PackHasher
	audit  *Audit
	clock  Clock
	log    *slog.Logger
}

func NewPacks(repo PackRepo, hasher PackHasher, audit *Audit, clock Clock, log *slog.Logger) *Packs {
	return &Packs{repo: repo, hasher: hasher, audit: audit, clock: clock, log: log}
}

func (p *Packs) List(ctx context.Context, actor *domain.User) ([]domain.Pack, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}
	return p.repo.List(ctx)
}

// Create anade un paquete a la biblioteca.
func (p *Packs) Create(ctx context.Context, actor *domain.User, nombre, url, nota, ip string) (*domain.Pack, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}

	nombre, url, nota = strings.TrimSpace(nombre), strings.TrimSpace(url), strings.TrimSpace(nota)
	if nombre == "" {
		return nil, domain.ErrEmptyName
	}
	if !domain.PackURLValida(url) {
		return nil, domain.ErrPackURLInvalida
	}

	pack := &domain.Pack{
		Name:      nombre,
		URL:       url,
		Note:      nota,
		SHA1:      p.hashDe(ctx, url),
		CreatedBy: actor.ID,
		CreatedAt: p.clock(),
	}

	id, err := p.repo.Create(ctx, pack)
	if err != nil {
		return nil, err
	}
	pack.ID = id

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPackCreated, pack.Name, ip)
	return pack, nil
}

// Update cambia el nombre, el enlace o la nota.
func (p *Packs) Update(ctx context.Context, actor *domain.User, id int64, nombre, url, nota, ip string) error {
	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	pack, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}

	nombre, url, nota = strings.TrimSpace(nombre), strings.TrimSpace(url), strings.TrimSpace(nota)
	if nombre == "" {
		return domain.ErrEmptyName
	}
	if !domain.PackURLValida(url) {
		return domain.ErrPackURLInvalida
	}

	// El hash solo se recalcula si el enlace cambio. Volver a descargar el
	// archivo por corregir una falta de ortografia en el nombre seria pagar
	// una descarga por nada.
	if url != pack.URL {
		pack.SHA1 = p.hashDe(ctx, url)
	}

	pack.Name, pack.URL, pack.Note = nombre, url, nota
	if err := p.repo.Update(ctx, pack); err != nil {
		return err
	}

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPackUpdated, pack.Name, ip)
	return nil
}

func (p *Packs) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermWorldDelete) {
		return domain.ErrForbidden
	}

	pack, err := p.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	// Las asignaciones se van en cascada con el: un paquete borrado no puede
	// seguir figurando en la lista de ningun mundo.
	if err := p.repo.Delete(ctx, id); err != nil {
		return err
	}

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPackDeleted, pack.Name, ip)
	return nil
}

// DeMundo lista los paquetes de un mundo.
func (p *Packs) DeMundo(ctx context.Context, actor *domain.User, worldID int64) ([]domain.PackAsignado, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}
	return p.repo.DeMundo(ctx, worldID)
}

// Asignar fija los paquetes de un mundo y cual se aplica solo.
//
// activo puede ser 0: un mundo puede llevar paquetes listados para que la gente
// los instale a mano sin que ninguno se aplique automaticamente.
func (p *Packs) Asignar(ctx context.Context, actor *domain.User, worldID int64,
	ids []int64, activo int64, requerido bool, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	if activo != 0 {
		if !contiene(ids, activo) {
			return domain.ErrPackActivoNoAsignado
		}
		// Un enlace a una pagina de descarga no se puede aplicar solo: el
		// cliente pediria el archivo y recibiria HTML. Se rechaza al elegirlo y
		// no al arrancar, porque aqui todavia hay una pantalla donde decirlo.
		pack, err := p.repo.ByID(ctx, activo)
		if err != nil {
			return err
		}
		if !pack.Automatico() {
			return domain.ErrPackNoAutomatico
		}
	}

	if err := p.repo.Asignar(ctx, worldID, ids, activo); err != nil {
		return err
	}
	if err := p.repo.SetRequired(ctx, worldID, requerido && activo != 0); err != nil {
		return err
	}

	p.audit.Record(ctx, actor, actor.Email, domain.ActionPackAssigned, "", ip)
	return nil
}

// ActivoDe es lo que lee la instancia al arrancar.
func (p *Packs) ActivoDe(ctx context.Context, worldID int64) (domain.PackRef, error) {
	return p.repo.ActivoDe(ctx, worldID)
}

// hashDe intenta calcular el SHA-1, y se conforma con no tenerlo.
//
// Que falle no impide anadir el paquete: sin hash el cliente se lo descarga en
// cada conexion, que es peor pero funciona. Bloquear el alta porque un servidor
// ajeno no responde seria dejar de hacer lo unico que el panel si controla.
func (p *Packs) hashDe(ctx context.Context, url string) string {
	if p.hasher == nil {
		return ""
	}

	sha, err := p.hasher.SHA1(ctx, url)
	if err != nil {
		p.log.Warn("no se pudo calcular el hash del paquete; se descargara en cada conexion",
			"url", url, "error", err)
		return ""
	}
	return sha
}

func contiene(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
