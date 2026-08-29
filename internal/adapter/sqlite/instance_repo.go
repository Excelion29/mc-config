package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// InstanceRepo implementa app.InstanceRepo.
type InstanceRepo struct{ db *sql.DB }

const instanceColumns = `i.id, i.name, i.slug, i.edition, i.version, i.world_id,
	i.level_name, i.container_id, i.spec_hash, i.port, i.state, i.memory_mb, i.cpus,
	i.created_at, i.last_started, COALESCE(w.name, '')`

const instanceFrom = `FROM instances i LEFT JOIN worlds w ON w.id = i.world_id`

func (r *InstanceRepo) Create(ctx context.Context, i *domain.Instance) (int64, error) {
	var worldID any
	if i.WorldID > 0 {
		worldID = i.WorldID
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO instances (name, slug, edition, version, world_id, level_name,
		                        container_id, port, state, memory_mb, cpus, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.Name, i.Slug, string(i.Edition), i.Version, worldID, i.LevelName,
		i.ContainerID, i.Port, string(i.State), i.MemoryMB, i.CPUs,
		i.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, domain.ErrDuplicateInstance
		}
		return 0, fmt.Errorf("insertando instancia: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("leyendo id de la instancia: %w", err)
	}
	return id, nil
}

func (r *InstanceRepo) ByID(ctx context.Context, id int64) (*domain.Instance, error) {
	return r.scanOne(ctx, `SELECT `+instanceColumns+` `+instanceFrom+` WHERE i.id = ?`, id)
}

func (r *InstanceRepo) BySlug(ctx context.Context, slug string) (*domain.Instance, error) {
	return r.scanOne(ctx, `SELECT `+instanceColumns+` `+instanceFrom+` WHERE i.slug = ?`, slug)
}

// Running devuelve la instancia encendida. Por D-02 solo puede haber una, pero
// se ordena por estado para que una en transicion no pase desapercibida.
func (r *InstanceRepo) Running(ctx context.Context) (*domain.Instance, error) {
	return r.scanOne(ctx,
		`SELECT `+instanceColumns+` `+instanceFrom+`
		 WHERE i.state IN ('running','starting','stopping')
		 ORDER BY CASE i.state WHEN 'running' THEN 0 ELSE 1 END, i.id
		 LIMIT 1`)
}

func (r *InstanceRepo) List(ctx context.Context) ([]domain.Instance, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+instanceColumns+` `+instanceFrom+` ORDER BY i.created_at DESC, i.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("listando instancias: %w", err)
	}
	defer rows.Close()

	var out []domain.Instance
	for rows.Next() {
		inst, err := scanInstance(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *inst)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo instancias: %w", err)
	}
	return out, nil
}

func (r *InstanceRepo) SetState(ctx context.Context, id int64, state domain.InstanceState) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE instances SET state = ? WHERE id = ?`, string(state), id)
	if err != nil {
		return fmt.Errorf("cambiando el estado: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrInstanceNotFound
	}
	return nil
}

func (r *InstanceRepo) SetContainer(ctx context.Context, id int64, containerID, specHash string) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE instances SET container_id = ?, spec_hash = ? WHERE id = ?`,
		containerID, specHash, id); err != nil {
		return fmt.Errorf("guardando el contenedor: %w", err)
	}
	return nil
}

func (r *InstanceRepo) MarkStarted(ctx context.Context, id int64, at time.Time) error {
	if _, err := r.db.ExecContext(ctx,
		`UPDATE instances SET last_started = ? WHERE id = ?`,
		at.Format(time.RFC3339), id); err != nil {
		return fmt.Errorf("marcando el arranque: %w", err)
	}
	return nil
}

func (r *InstanceRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("borrando la instancia: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrInstanceNotFound
	}
	return nil
}

// CountByWorld se usa para avisar antes de borrar un mapa que alguna instancia
// esta usando.
func (r *InstanceRepo) CountByWorld(ctx context.Context, worldID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM instances WHERE world_id = ?`, worldID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("contando instancias del mapa: %w", err)
	}
	return n, nil
}

func (r *InstanceRepo) scanOne(ctx context.Context, query string, args ...any) (*domain.Instance, error) {
	inst, err := scanInstance(r.db.QueryRowContext(ctx, query, args...).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInstanceNotFound
	}
	return inst, err
}

func scanInstance(scan func(...any) error) (*domain.Instance, error) {
	var (
		i           domain.Instance
		edition     string
		state       string
		worldID       sql.NullInt64
		createdAt   string
		lastStarted sql.NullString
	)
	err := scan(&i.ID, &i.Name, &i.Slug, &edition, &i.Version, &worldID,
		&i.LevelName, &i.ContainerID, &i.SpecHash, &i.Port, &state, &i.MemoryMB, &i.CPUs,
		&createdAt, &lastStarted, &i.WorldName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("leyendo instancia: %w", err)
	}

	i.Edition = domain.Edition(edition)
	i.State = domain.InstanceState(state)
	if worldID.Valid {
		i.WorldID = worldID.Int64
	}
	i.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastStarted.Valid {
		if t, err := time.Parse(time.RFC3339, lastStarted.String); err == nil {
			i.LastStarted = &t
		}
	}
	return &i, nil
}
