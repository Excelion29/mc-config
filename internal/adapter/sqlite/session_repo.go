package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// SessionRepo implementa app.SessionRepo.
type SessionRepo struct{ db *sql.DB }

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		s.Token, s.UserID, s.CreatedAt.Format(time.RFC3339), s.ExpiresAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("creando sesion: %w", err)
	}
	return nil
}

func (r *SessionRepo) ByToken(ctx context.Context, token string) (*domain.Session, error) {
	var (
		s         domain.Session
		createdAt string
		expiresAt string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT token, user_id, created_at, expires_at FROM sessions WHERE token = ?`, token).
		Scan(&s.Token, &s.UserID, &createdAt, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo sesion: %w", err)
	}

	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		// Una fecha ilegible se trata como sesion invalida, no como error:
		// nunca debe dar acceso por no poder interpretarla.
		return nil, domain.ErrNotFound
	}
	return &s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("borrando sesion: %w", err)
	}
	return nil
}

func (r *SessionRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, now.Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("purgando sesiones: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
