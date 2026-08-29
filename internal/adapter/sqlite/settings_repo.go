package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// SettingsRepo guarda los ajustes globales del panel.
type SettingsRepo struct {
	db *sql.DB
}

// Get devuelve el valor de un ajuste. Si no existe, devuelve el que se le pase
// por defecto: un ajuste que falta no es un error, es uno que nadie ha tocado.
func (r *SettingsRepo) Get(ctx context.Context, clave, porDefecto string) (string, error) {
	var v string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, clave).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return porDefecto, nil
	}
	if err != nil {
		return "", fmt.Errorf("leyendo el ajuste %s: %w", clave, err)
	}
	return v, nil
}

// Set guarda un ajuste, creandolo si no estaba.
func (r *SettingsRepo) Set(ctx context.Context, clave, valor string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value,
		                                updated_at = excluded.updated_at`,
		clave, valor, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("guardando el ajuste %s: %w", clave, err)
	}
	return nil
}

var _ = domain.SettingAuthMode
