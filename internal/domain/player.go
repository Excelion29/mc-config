package domain

import (
	"strings"
	"time"
)

// Player es un amigo autorizado a entrar a los servidores (D-13).
//
// No es un usuario del panel: son dos identidades distintas y no se mezclan.
// Un jugador puede no tener cuenta en el panel, y al reves.
type Player struct {
	ID int64
	// Gamertag es el nombre de Xbox Live, tal cual. Bedrock lo compara
	// EXACTO: mayusculas y espacios incluidos.
	Gamertag string
	// JavaName se rellenara en el hito 2. La identidad de Java es distinta:
	// usuario y contrasena de AuthMe, no gamertag (ver D-04).
	JavaName string
	Note     string
	// IsOp da permisos de administrador DENTRO del juego, no en el panel.
	IsOp      bool
	Active    bool
	CreatedAt time.Time
}

// NormalizeGamertag limpia espacios sobrantes pero CONSERVA las mayusculas.
//
// Bedrock compara el gamertag exacto: pasarlo a minusculas dejaria fuera a la
// persona sin decir por que.
func NormalizeGamertag(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
