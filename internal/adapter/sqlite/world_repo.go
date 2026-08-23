package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// WorldRepo implementa app.WorldRepo.
type WorldRepo struct{ db *sql.DB }

const worldColumns = `id, name, raw_name, edition, version, file_name,
	size_bytes, sha256, has_icon, uploaded_by, created_at`

func (r *WorldRepo) Create(ctx context.Context, m *domain.World) (int64, error) {
	var uploadedBy any
	if m.UploadedBy > 0 {
		uploadedBy = m.UploadedBy
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO worlds (name, raw_name, edition, version, file_name,
		                   size_bytes, sha256, has_icon, uploaded_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Name, m.RawName, string(m.Edition), m.Version, m.FileName,
		m.SizeBytes, m.SHA256, boolToInt(m.HasIcon), uploadedBy,
		m.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, domain.ErrDuplicateWorld
		}
		return 0, fmt.Errorf("insertando mapa: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("leyendo id del mapa: %w", err)
	}
	return id, nil
}

func (r *WorldRepo) ByID(ctx context.Context, id int64) (*domain.World, error) {
	return r.scanOne(ctx, `SELECT `+worldColumns+` FROM worlds WHERE id = ?`, id)
}

func (r *WorldRepo) BySHA(ctx context.Context, sha string) (*domain.World, error) {
	return r.scanOne(ctx, `SELECT `+worldColumns+` FROM worlds WHERE sha256 = ?`, sha)
}

func (r *WorldRepo) List(ctx context.Context) ([]domain.World, error) {
	out, _, err := r.ListPage(ctx, -1, 0)
	return out, err
}

// ListPage devuelve una pagina y cuantas filas hay en total.
//
// El total viaja junto a las filas porque quien pinta el paginador lo
// necesita, y pedirlo en otra llamada abriria la puerta a que las dos
// consultas vieran estados distintos de la tabla.
//
// Un limite <= 0 significa "sin limite": SQLite lo entiende como LIMIT -1. Asi
// List sigue siendo un caso particular de esto y no hay dos consultas que
// mantener sincronizadas.
func (r *WorldRepo) ListPage(ctx context.Context, limit, offset int) ([]domain.World, int, error) {
	if limit <= 0 {
		limit = -1
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM worlds`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando mundos: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+worldColumns+` FROM worlds ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("listando mapas: %w", err)
	}
	defer rows.Close()

	var maps []domain.World
	for rows.Next() {
		m, err := scanWorld(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		maps = append(maps, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("recorriendo mapas: %w", err)
	}
	return maps, total, nil
}

func (r *WorldRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM worlds WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("borrando mapa: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrWorldNotFound
	}
	return nil
}

func (r *WorldRepo) scanOne(ctx context.Context, query string, args ...any) (*domain.World, error) {
	m, err := scanWorld(r.db.QueryRowContext(ctx, query, args...).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrWorldNotFound
	}
	return m, err
}

func scanWorld(scan func(...any) error) (*domain.World, error) {
	var (
		m          domain.World
		edition    string
		hasIcon    int
		uploadedBy sql.NullInt64
		createdAt  string
	)
	err := scan(&m.ID, &m.Name, &m.RawName, &edition, &m.Version, &m.FileName,
		&m.SizeBytes, &m.SHA256, &hasIcon, &uploadedBy, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("leyendo mapa: %w", err)
	}

	m.Edition = domain.Edition(edition)
	m.HasIcon = hasIcon == 1
	if uploadedBy.Valid {
		m.UploadedBy = uploadedBy.Int64
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &m, nil
}
