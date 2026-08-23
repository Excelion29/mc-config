package app

import (
	"context"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Puerto de la lista maestra de jugadores.

// PlayerRepo persiste la lista maestra de jugadores (D-13).
type PlayerRepo interface {
	Create(ctx context.Context, p *domain.Player) (int64, error)
	ByID(ctx context.Context, id int64) (*domain.Player, error)
	List(ctx context.Context) ([]domain.Player, error)
	ListPage(ctx context.Context, limit, offset int) ([]domain.Player, int, error)
	ActiveGamertags(ctx context.Context) ([]string, error)
	SetActive(ctx context.Context, id int64, active bool) error
	SetOp(ctx context.Context, id int64, isOp bool) error
	Delete(ctx context.Context, id int64) error
}
