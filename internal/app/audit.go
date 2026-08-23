package app

import (
	"context"
	"log/slog"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Audit registra lo que ocurre en el panel (D-08).
type Audit struct {
	repo  AuditRepo
	clock Clock
	log   *slog.Logger
}

func NewAudit(repo AuditRepo, clock Clock, log *slog.Logger) *Audit {
	return &Audit{repo: repo, clock: clock, log: log}
}

// Record anota una accion.
//
// No devuelve error a proposito: si fallara el registro no queremos abortar la
// accion del usuario, pero tampoco perder el aviso. Se deja en el log del
// proceso para que sea visible.
func (a *Audit) Record(ctx context.Context, actor *domain.User, email string, action domain.Action, detail, ip string) {
	e := &domain.LogEntry{
		UserEmail: email,
		Action:    action,
		Detail:    detail,
		IP:        ip,
		CreatedAt: a.clock(),
	}
	if actor != nil {
		id := actor.ID
		e.UserID = &id
		e.UserEmail = actor.Email
	}

	if err := a.repo.Record(ctx, e); err != nil {
		a.log.Error("no se pudo registrar la accion",
			"accion", action, "email", e.UserEmail, "error", err)
	}
}

func (a *Audit) Latest(ctx context.Context, limit int) ([]domain.LogEntry, error) {
	return a.repo.Latest(ctx, limit)
}
