package app

import (
	"context"

	"github.com/Excelion29/mc-config/internal/domain"
)

// AuditFilter acota una consulta al registro.
//
// El campo cero es "sin filtrar", asi que una consulta vacia sigue devolviendo
// las ultimas acciones y no hace falta un caso especial en la pagina.
type AuditFilter struct {
	// Text busca en el correo y en el detalle a la vez. Es un solo campo a
	// proposito: quien mira el registro busca "wronkow" o "luckyblocks" sin
	// pararse a pensar en cual de las dos columnas cae.
	Text string
	// Action, si no esta vacia, exige una accion exacta.
	Action domain.Action
	// Page empieza en 1.
	Page int
	// Size es cuantas filas por pagina.
	Size int
}

// Normalize delega en Paging: la regla de que es una pagina valida es una
// sola para toda la aplicacion.
func (f AuditFilter) Normalize() AuditFilter {
	pg := Paging{Page: f.Page, Size: f.Size}.Normalize(25, 200)
	f.Page, f.Size = pg.Page, pg.Size
	return f
}

func (f AuditFilter) Offset() int { return (f.Page - 1) * f.Size }

// AuditPage es una pagina del registro junto con lo que hace falta para pintar
// el paginador.
type AuditPage struct {
	Entries []domain.LogEntry
	PageInfo
	Filter AuditFilter
}

// Search devuelve una pagina del registro.
func (a *Audit) Search(ctx context.Context, actor *domain.User, f AuditFilter) (AuditPage, error) {
	if !actor.Can(domain.PermAuditView) {
		return AuditPage{}, domain.ErrForbidden
	}

	f = f.Normalize()
	entries, total, err := a.repo.Search(ctx, f.Text, f.Action, f.Size, f.Offset())
	if err != nil {
		return AuditPage{}, err
	}

	return AuditPage{
		Entries:  entries,
		PageInfo: NewPageInfo(Paging{Page: f.Page, Size: f.Size}, total),
		Filter:   f,
	}, nil
}

// AuditActions es el catalogo de acciones para el desplegable del filtro.
//
// Sale del codigo y no de un "SELECT DISTINCT" a proposito: asi el desplegable
// ofrece siempre las mismas opciones, incluso las que aun no ha hecho nadie.
// Una lista que cambia segun lo que ya paso es una lista en la que no se puede
// confiar para buscar lo que NO paso.
func AuditActions() []domain.Action {
	return []domain.Action{
		domain.ActionLogin, domain.ActionLoginFailed, domain.ActionLogout,
		domain.ActionUserCreated, domain.ActionUserEnabled,
		domain.ActionUserDisabled, domain.ActionUserRoleChanged,
		domain.ActionRoleCreated, domain.ActionRoleUpdated, domain.ActionRoleDeleted,
		domain.ActionWorldImported, domain.ActionWorldCreated,
		domain.ActionWorldUpdated, domain.ActionWorldDeleted,
		domain.ActionInstanceCreated, domain.ActionInstanceStarted,
		domain.ActionInstanceStopped, domain.ActionInstanceDeleted,
		domain.ActionPlayerAdded, domain.ActionPlayerRemoved,
		domain.ActionPlayerEnabled, domain.ActionPlayerDisabled,
		domain.ActionPlayerOp,
	}
}
