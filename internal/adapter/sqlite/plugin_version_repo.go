package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// PluginVersionRepo implementa app.PluginVersionRepo.
type PluginVersionRepo struct{ db *sql.DB }

// All devuelve solo lo que alguien cambio. Sin fila, manda el codigo.
func (r *PluginVersionRepo) All(ctx context.Context) (map[string]app.PluginVersion, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT plugin_id, url, file FROM plugin_versions`)
	if err != nil {
		return nil, fmt.Errorf("leyendo las versiones de los complementos: %w", err)
	}
	defer rows.Close()

	out := map[string]app.PluginVersion{}
	for rows.Next() {
		var v app.PluginVersion
		if err := rows.Scan(&v.PluginID, &v.URL, &v.File); err != nil {
			return nil, err
		}
		out[v.PluginID] = v
	}
	return out, rows.Err()
}

func (r *PluginVersionRepo) Set(ctx context.Context, v app.PluginVersion, by int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO plugin_versions (plugin_id, url, file, updated_by, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(plugin_id) DO UPDATE SET
		     url = excluded.url, file = excluded.file,
		     updated_by = excluded.updated_by, updated_at = excluded.updated_at`,
		v.PluginID, v.URL, v.File, by, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("guardando la version del complemento: %w", err)
	}
	return nil
}

// Clear devuelve un complemento a la version que trae el codigo.
func (r *PluginVersionRepo) Clear(ctx context.Context, pluginID string) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM plugin_versions WHERE plugin_id = ?`, pluginID); err != nil {
		return fmt.Errorf("volviendo a la version de fabrica: %w", err)
	}
	return nil
}
