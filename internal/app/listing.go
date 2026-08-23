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

// PlayersPage es una pagina de la lista maestra de jugadores.
type PlayersPage struct {
	Players []domain.Player
	PageInfo
}

func (p *Players) ListPage(ctx context.Context, actor *domain.User, pg Paging) (PlayersPage, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return PlayersPage{}, domain.ErrForbidden
	}

	pg = pg.Normalize(25, 200)
	list, total, err := p.repo.ListPage(ctx, pg.Size, pg.Offset())
	if err != nil {
		return PlayersPage{}, err
	}
	return PlayersPage{Players: list, PageInfo: NewPageInfo(pg, total)}, nil
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
