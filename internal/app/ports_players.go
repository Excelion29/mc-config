package app

import (
	"context"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Puerto de la lista maestra de jugadores.

// PlayerRepo persiste la lista maestra de jugadores (D-13).
type PlayerRepo interface {
	Create(ctx context.Context, p *domain.Player) (int64, error)
	ByID(ctx context.Context, id int64) (*domain.Player, error)
	List(ctx context.Context) ([]domain.Player, error)
	ListPage(ctx context.Context, limit, offset int) ([]domain.Player, int, error)
	// SearchPage filtra por texto libre y por estado.
	SearchPage(ctx context.Context, texto, estado string, limit, offset int) ([]domain.Player, int, error)
	ActiveGamertags(ctx context.Context) ([]string, error)
	Permitidos(ctx context.Context) ([]domain.Player, error)
	SetActive(ctx context.Context, id int64, active bool) error
	SetOp(ctx context.Context, id int64, isOp bool) error
	Delete(ctx context.Context, id int64) error
	// MarkSeen guarda el XUID la primera vez que se ve entrar a alguien.
	// Devuelve si de verdad cambio algo.
	MarkSeen(ctx context.Context, gamertag, xuid string, cuando time.Time) (bool, error)
	// Ops devuelve los operadores IDENTIFICABLES, es decir con XUID.
	Ops(ctx context.Context) ([]domain.Player, error)
}
