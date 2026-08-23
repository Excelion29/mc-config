package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

	return scanEntries(rows, limit)
}

// Search filtra y pagina el registro.
//
// El total se cuenta con la MISMA clausula WHERE que las filas y en la misma
// conexion, para que el paginador no diga "312 resultados" mientras la pagina
// muestra otra cosa.
func (r *AuditRepo) Search(ctx context.Context, text string, action domain.Action, limit, offset int) ([]domain.LogEntry, int, error) {
	// Las condiciones se acumulan como texto pero los valores SIEMPRE van
	// como parametros: lo que escribe el usuario nunca se concatena al SQL.
	where := "WHERE 1=1"
	var args []any

	if t := strings.TrimSpace(text); t != "" {
		// Se busca en las dos columnas a la vez porque quien consulta no
		// piensa en columnas, piensa en "algo que ponga wronkow".
		where += " AND (user_email LIKE ? ESCAPE '\\' OR detail LIKE ? ESCAPE '\\')"
		patron := "%" + escapeLike(t) + "%"
		args = append(args, patron, patron)
	}
	if action != "" {
		where += " AND action = ?"
		args = append(args, string(action))
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM audit_log "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando el registro: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, user_email, action, detail, ip, created_at
		   FROM audit_log `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("leyendo el registro: %w", err)
	}
	defer rows.Close()

	entries, err := scanEntries(rows, limit)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// escapeLike neutraliza los comodines de LIKE.
//
// Sin esto, buscar "100%" devolveria cualquier cosa que empiece por "100", y
// un "_" solo casaria con todo. Son caracteres normales en un correo o en el
// nombre de un mapa, asi que hay que tratarlos como texto.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func scanEntries(rows *sql.Rows, capacidad int) ([]domain.LogEntry, error) {
	entries := make([]domain.LogEntry, 0, capacidad)
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
