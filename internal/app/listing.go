package app

import (
	"context"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Este archivo reune las variantes paginadas de los listados.
//
// Todas siguen la misma forma -permiso, normalizar, consultar, empaquetar- y
// devuelven la misma PageInfo, de manera que la plantilla del paginador es una
// sola para todas las pantallas. Cuando cada listado inventa su propio contrato
// acaban discrepando en los casos limite, que es justo donde importa.

// PlayerFilter acota la lista maestra.
type PlayerFilter struct {
	// Text busca en el gamertag y en la nota a la vez.
	Text string
	// Estado es uno de EstadosDeJugador. Vacio = todos.
	Estado string
}

// EstadosDeJugador son las opciones del desplegable, en orden.
//
// Salen del codigo y no de la base: asi el filtro ofrece siempre las mismas
// opciones. Una lista construida con lo que ya existe no permite buscar lo que
// NO existe, que suele ser justo la pregunta -"a quien le falta entrar?"-.
func EstadosDeJugador() [][2]string {
	return [][2]string{
		{"", "Todos"},
		{"activos", "Activos"},
		{"bloqueados", "Bloqueados"},
		{"sin-estrenar", "Sin estrenar"},
		{"admins", "Admins del juego"},
	}
}

// PlayersPage es una pagina de la lista maestra de jugadores.
type PlayersPage struct {
	Players []domain.Player
	PageInfo
	Filter PlayerFilter
}

func (p *Players) ListPage(ctx context.Context, actor *domain.User, pg Paging) (PlayersPage, error) {
	return p.SearchPage(ctx, actor, PlayerFilter{}, pg)
}

func (p *Players) SearchPage(ctx context.Context, actor *domain.User, f PlayerFilter, pg Paging) (PlayersPage, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return PlayersPage{}, domain.ErrForbidden
	}

	pg = pg.Normalize(25, 200)
	list, total, err := p.repo.SearchPage(ctx, f.Text, f.Estado, pg.Size, pg.Offset())
	if err != nil {
		return PlayersPage{}, err
	}
	return PlayersPage{Players: list, PageInfo: NewPageInfo(pg, total), Filter: f}, nil
}

// MapsPage es una pagina de la biblioteca.
type MapsPage struct {
	Maps []domain.Map
	PageInfo
}

func (m *Maps) ListPage(ctx context.Context, actor *domain.User, pg Paging) (MapsPage, error) {
	if !actor.Can(domain.PermServerView) {
		return MapsPage{}, domain.ErrForbidden
	}

	// Los mapas se pintan con portada, asi que caben menos por pagina que
	// filas de una tabla.
	pg = pg.Normalize(12, 100)
	list, total, err := m.repo.ListPage(ctx, pg.Size, pg.Offset())
	if err != nil {
		return MapsPage{}, err
	}
	return MapsPage{Maps: list, PageInfo: NewPageInfo(pg, total)}, nil
}

// UsersPage es una pagina de las cuentas del panel.
type UsersPage struct {
	Users []domain.User
	PageInfo
}

func (a *Auth) ListUsersPage(ctx context.Context, actor *domain.User, pg Paging) (UsersPage, error) {
	if !actor.Can(domain.PermUserManage) {
		return UsersPage{}, domain.ErrForbidden
	}

	pg = pg.Normalize(25, 200)
	list, total, err := a.users.ListPage(ctx, pg.Size, pg.Offset())
	if err != nil {
		return UsersPage{}, err
	}
	return UsersPage{Users: list, PageInfo: NewPageInfo(pg, total)}, nil
}
