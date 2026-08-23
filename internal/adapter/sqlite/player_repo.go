package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// PlayerRepo implementa app.PlayerRepo.
type PlayerRepo struct{ db *sql.DB }

const playerColumns = `id, gamertag, java_name, note, is_op, active, created_at`

func (r *PlayerRepo) Create(ctx context.Context, p *domain.Player) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO players (gamertag, java_name, note, is_op, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.Gamertag, p.JavaName, p.Note, boolToInt(p.IsOp), boolToInt(p.Active),
		p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, domain.ErrDuplicatePlayer
		}
		return 0, fmt.Errorf("insertando jugador: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("leyendo id del jugador: %w", err)
	}
	return id, nil
}

func (r *PlayerRepo) ByID(ctx context.Context, id int64) (*domain.Player, error) {
	p, err := scanPlayer(r.db.QueryRowContext(ctx,
		`SELECT `+playerColumns+` FROM players WHERE id = ?`, id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPlayerNotFound
	}
	return p, err
}

func (r *PlayerRepo) List(ctx context.Context) ([]domain.Player, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playerColumns+` FROM players ORDER BY active DESC, gamertag ASC`)
	if err != nil {
		return nil, fmt.Errorf("listando jugadores: %w", err)
	}
	defer rows.Close()

	var out []domain.Player
	for rows.Next() {
		p, err := scanPlayer(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// ActiveGamertags es lo que acaba dentro de allowlist.json.
func (r *PlayerRepo) ActiveGamertags(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT gamertag FROM players WHERE active = 1 ORDER BY gamertag`)
	if err != nil {
		return nil, fmt.Errorf("leyendo jugadores activos: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, fmt.Errorf("leyendo gamertag: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *PlayerRepo) SetActive(ctx context.Context, id int64, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE players SET active = ? WHERE id = ?`, boolToInt(active), id)
	if err != nil {
		return fmt.Errorf("cambiando estado del jugador: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrPlayerNotFound
	}
	return nil
}

func (r *PlayerRepo) SetOp(ctx context.Context, id int64, isOp bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE players SET is_op = ? WHERE id = ?`, boolToInt(isOp), id)
	if err != nil {
		return fmt.Errorf("cambiando operador: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrPlayerNotFound
	}
	return nil
}

func (r *PlayerRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("borrando jugador: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrPlayerNotFound
	}
	return nil
}

func scanPlayer(scan func(...any) error) (*domain.Player, error) {
	var (
		p         domain.Player
		isOp      int
		active    int
		createdAt string
	)
	err := scan(&p.ID, &p.Gamertag, &p.JavaName, &p.Note, &isOp, &active, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("leyendo jugador: %w", err)
	}

	p.IsOp = isOp == 1
	p.Active = active == 1
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}
