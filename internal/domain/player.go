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
	// JavaName es el nombre de Minecraft Java. Es una TERCERA identidad,
	// distinta del gamertag de Bedrock y de la cuenta del panel: la misma
	// persona se llama distinto en cada una (H-J-9).
	JavaName string
	// JavaUUID identifica a la persona ante un servidor Java, que no acepta
	// nombres en su whitelist.
	//
	// A diferencia del XUID de Bedrock, este SI se puede resolver antes de que
	// nadie juegue: se le pregunta a Mojang a partir del nombre.
	JavaUUID string
	Note string
	// XUID es el identificador de Xbox Live. Ya existe antes de que nos
	// conozca -lo tiene desde que creo su cuenta- pero el panel no lo sabe
	// hasta que entra por primera vez y el servidor lo escribe en su log.
	//
	// Hace falta porque permissions.json identifica a los operadores por XUID
	// y NO acepta gamertags.
	XUID string
	// FirstSeen es cuando se le vio entrar por primera vez. Nil = nunca.
	FirstSeen *time.Time
	// IsOp da permisos de administrador DENTRO del juego, no en el panel.
	//
	// Solo surte efecto si ademas hay XUID: hasta entonces el servidor no
	// sabe a quien se refiere. Por eso la interfaz no ofrece la opcion antes
	// de la primera conexion, en vez de dejar marcar algo que no haria nada.
	IsOp      bool
	Active    bool
	CreatedAt time.Time
}

// HaEntrado indica si se le ha visto conectarse alguna vez.
//
// Es la frontera del alta en dos fases: antes solo se le puede permitir o
// bloquear el paso; despues ya se le puede gestionar de verdad.
func (p *Player) HaEntrado() bool {
	return p != nil && p.XUID != ""
}

// PuedeSerOp: dar operador exige saber quien es exactamente.
func (p *Player) PuedeSerOp() bool {
	return p.HaEntrado()
}

// NormalizeGamertag limpia espacios sobrantes pero CONSERVA las mayusculas.
//
// Bedrock compara el gamertag exacto: pasarlo a minusculas dejaria fuera a la
// persona sin decir por que.
func NormalizeGamertag(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// PuedeJugarBedrock indica si tiene identidad para entrar a un servidor
// Bedrock. Basta el gamertag: la allow-list de Bedrock va por nombre.
func (p *Player) PuedeJugarBedrock() bool {
	return p != nil && p.Gamertag != ""
}

// PuedeJugarJava exige el UUID, porque whitelist.json no acepta nombres.
func (p *Player) PuedeJugarJava() bool {
	return p != nil && p.JavaUUID != ""
}
