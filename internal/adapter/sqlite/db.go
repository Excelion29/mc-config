// Package sqlite implementa los puertos de persistencia de app sobre SQLite (D-14).
//
// Todo lo especifico del motor vive aqui. Migrar a PostgreSQL significaria
// escribir un paquete hermano, sin tocar domain ni app.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/pressly/goose/v3"

	// Driver SQLite en Go puro: sin cgo, para que el binario siga siendo
	// estatico. mattn/go-sqlite3 exige cgo y rompe esa propiedad (D-12).
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB envuelve la conexion y expone los repositorios.
type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	// journal_mode(WAL) es obligatorio por D-14: sin el, una escritura bloquea
	// todas las lecturas.
	// busy_timeout evita que una colision puntual falle al instante.
	// foreign_keys viene APAGADO por defecto en SQLite; sin esto las claves
	// foraneas del esquema serian decorativas.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)",
		url.PathEscape(filepath.ToSlash(path)),
	)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo sqlite: %w", err)
	}

	// SQLite admite un unico escritor. Con una sola conexion se serializan las
	// escrituras y desaparece "database is locked". Con el volumen de este
	// panel (cientos de filas) no se nota.
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("conectando a sqlite: %w", err)
	}

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}

	return &DB{db: conn}, nil
}

func migrate(conn *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("fijando dialecto de goose: %w", err)
	}
	if err := goose.Up(conn, "migrations"); err != nil {
		return fmt.Errorf("aplicando migraciones: %w", err)
	}
	return nil
}

func (d *DB) Close() error { return d.db.Close() }

// AppliedMigrations devuelve que versiones registro goose.
//
// goose lleva su propia contabilidad en goose_db_version. Vaciar esa tabla a
// mano deja la base inconsistente: las tablas existen pero el registro dice que
// no. Para reiniciar de verdad hay que borrar el archivo, o eliminar TODAS las
// tablas incluida esta.
func (d *DB) AppliedMigrations(ctx context.Context) (map[int]bool, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT version_id, is_applied FROM goose_db_version ORDER BY version_id`)
	if err != nil {
		return nil, fmt.Errorf("leyendo el estado de las migraciones: %w", err)
	}
	defer rows.Close()

	out := map[int]bool{}
	for rows.Next() {
		var (
			version int
			applied int
		)
		if err := rows.Scan(&version, &applied); err != nil {
			return nil, fmt.Errorf("leyendo fila de migracion: %w", err)
		}
		out[version] = applied == 1
	}
	return out, rows.Err()
}

func (d *DB) Roles() *RoleRepo { return &RoleRepo{db: d.db} }

// Users necesita el repositorio de roles: al leer un usuario se carga su rol
// con los permisos, para que Can() responda sin volver a la base.
func (d *DB) Users() *UserRepo         { return &UserRepo{db: d.db, roles: d.Roles()} }
func (d *DB) Sessions() *SessionRepo   { return &SessionRepo{db: d.db} }
func (d *DB) Audit() *AuditRepo        { return &AuditRepo{db: d.db} }
func (d *DB) Worlds() *WorldRepo       { return &WorldRepo{db: d.db} }
func (d *DB) Settings() *SettingsRepo  { return &SettingsRepo{db: d.db} }
func (d *DB) Instances() *InstanceRepo { return &InstanceRepo{db: d.db} }
func (d *DB) Players() *PlayerRepo     { return &PlayerRepo{db: d.db} }
func (d *DB) Resources() *ResourceRepo { return &ResourceRepo{db: d.db} }
