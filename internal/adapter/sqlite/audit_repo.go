package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// AuditRepo implementa app.AuditRepo.
type AuditRepo struct{ db *sql.DB }

func (r *AuditRepo) Record(ctx context.Context, e *domain.LogEntry) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, user_email, action, detail, ip, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.UserID, e.UserEmail, string(e.Action), e.Detail, e.IP,
		e.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("registrando accion %q: %w", e.Action, err)
	}
	return nil
}

func (r *AuditRepo) Latest(ctx context.Context, limit int) ([]domain.LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, user_email, action, detail, ip, created_at
		   FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("leyendo el registro: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.LogEntry, 0, limit)
	for rows.Next() {
		var (
			e         domain.LogEntry
			userID    sql.NullInt64
			action    string
			createdAt string
		)
		if err := rows.Scan(&e.ID, &userID, &e.UserEmail, &action,
			&e.Detail, &e.IP, &createdAt); err != nil {
			return nil, fmt.Errorf("leyendo fila del registro: %w", err)
		}
		if userID.Valid {
			id := userID.Int64
			e.UserID = &id
		}
		e.Action = domain.Action(action)
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo el registro: %w", err)
	}
	return entries, nil
}
