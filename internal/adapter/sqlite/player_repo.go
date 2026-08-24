package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// PlayerRepo implementa app.PlayerRepo.
type PlayerRepo struct{ db *sql.DB }

const playerColumns = `id, gamertag, java_name, java_uuid, note, xuid, first_seen, is_op, active, created_at`

func (r *PlayerRepo) Create(ctx context.Context, p *domain.Player) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO players (gamertag, java_name, java_uuid, note, is_op, active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Gamertag, p.JavaName, p.JavaUUID, p.Note, boolToInt(p.IsOp), boolToInt(p.Active),
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
func (r *PlayerRepo) ListPage(ctx context.Context, limit, offset int) ([]domain.Player, int, error) {
	return r.SearchPage(ctx, "", "", limit, offset)
}

// SearchPage filtra y pagina la lista maestra.
//
// El texto busca en el gamertag Y en la nota a la vez, igual que en el
// registro: quien consulta piensa en "el del sotano", no en que columna
// guardo eso.
func (r *PlayerRepo) SearchPage(ctx context.Context, texto, estado string, limit, offset int) ([]domain.Player, int, error) {
	if limit <= 0 {
		limit = -1
	}

	where := "WHERE 1=1"
	var args []any

	if t := strings.TrimSpace(texto); t != "" {
		where += " AND (gamertag LIKE ? ESCAPE '\\' OR note LIKE ? ESCAPE '\\')"
		patron := "%" + escapeLike(t) + "%"
		args = append(args, patron, patron)
	}

	switch estado {
	case "activos":
		where += " AND active = 1"
	case "bloqueados":
		where += " AND active = 0"
	case "sin-estrenar":
		// Nunca se le ha visto entrar, asi que aun no se le puede hacer
		// admin del juego. Es el filtro que responde "a quien le falta dar
		// el primer paso".
		where += " AND xuid = ''"
	case "admins":
		where += " AND is_op = 1"
	}

	var total int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM players "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando players: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playerColumns+` FROM players `+where+
			` ORDER BY active DESC, gamertag ASC LIMIT ? OFFSET ?`,
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("listando jugadores: %w", err)
	}
	defer rows.Close()

	var out []domain.Player
	for rows.Next() {
		p, err := scanPlayer(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// Permitidos son los jugadores activos, enteros.
//
// Se devuelven completos y no solo sus nombres porque cada edicion identifica
// distinto: Bedrock por gamertag, Java por UUID. Quien construye el archivo
// decide que campo usar.
func (r *PlayerRepo) Permitidos(ctx context.Context) ([]domain.Player, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playerColumns+` FROM players WHERE active = 1 ORDER BY gamertag`)
	if err != nil {
		return nil, fmt.Errorf("leyendo jugadores permitidos: %w", err)
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
		firstSeen sql.NullString
	)
	err := scan(&p.ID, &p.Gamertag, &p.JavaName, &p.JavaUUID, &p.Note, &p.XUID, &firstSeen,
		&isOp, &active, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("leyendo jugador: %w", err)
	}

	p.IsOp = isOp == 1
	p.Active = active == 1
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if firstSeen.Valid {
		if t, err := time.Parse(time.RFC3339, firstSeen.String); err == nil {
			p.FirstSeen = &t
		}
	}
	return &p, nil
}

// MarkSeen guarda el XUID la PRIMERA vez que se ve entrar a alguien.
//
// El WHERE exige xuid = '' a proposito: solo se escribe una vez. Un jugador
// que entra veinte veces al dia no debe reescribir su fila veinte veces, y
// mas importante, first_seen tiene que seguir siendo la primera y no la
// ultima.
//
// Devuelve si de verdad se actualizo algo, para que quien llame sepa si hay
// que regenerar permissions.json o no ha cambiado nada.
func (r *PlayerRepo) MarkSeen(ctx context.Context, gamertag, xuid string, cuando time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE players SET xuid = ?, first_seen = ?
		  WHERE gamertag = ? AND xuid = ''`,
		xuid, cuando.Format(time.RFC3339), gamertag)
	if err != nil {
		return false, fmt.Errorf("guardando el xuid de %s: %w", gamertag, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Ops devuelve los jugadores que son operadores Y ademas se pueden identificar.
//
// El filtro por xuid distinto de vacio no es una optimizacion: permissions.json
// no admite gamertags, asi que un op sin XUID no se puede expresar. Se queda
// fuera hasta que entre por primera vez.
func (r *PlayerRepo) Ops(ctx context.Context) ([]domain.Player, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+playerColumns+` FROM players
		  WHERE is_op = 1 AND active = 1 AND xuid != '' ORDER BY gamertag`)
	if err != nil {
		return nil, fmt.Errorf("leyendo operadores: %w", err)
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
