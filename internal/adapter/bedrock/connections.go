package bedrock

import (
	"regexp"
	"strings"
)

// Connection es una conexion detectada en el log del servidor.
type Connection struct {
	Gamertag string
	// XUID es el identificador de Xbox Live. Ya existia antes -lo tiene la
	// persona desde que creo su cuenta-, pero el panel no lo conoce hasta que
	// se conecta por primera vez y el servidor lo escribe aqui.
	XUID string
}

// lineaConexion reconoce las variantes que emite el servidor de Bedrock.
//
// Se han visto al menos dos redacciones segun la version:
//
//	[... INFO] Player connected: Wronkow29, xuid: 2535413418839840
//	[... INFO] Player Spawned: Wronkow29 xuid: 2535413418839840, pfid: ...
//
// Por eso el patron acepta "connected" o "Spawned", la coma antes de xuid es
// opcional, y se corta en el primer campo siguiente. Es deliberadamente laxo:
// el formato lo decide Mojang y cambia entre versiones sin avisar, asi que
// atarse a una redaccion exacta es garantizar que un dia deje de funcionar.
//
// ATENCION: este formato NO esta verificado contra el servidor de la VPS.
// Sale de la documentacion y de versiones publicas. Por eso existe
// pareceConexion: cualquier linea que hable de un xuid y que este patron no
// entienda se registra como aviso, de modo que un cambio de formato se vea en
// los logs del panel en vez de traducirse en "la estrella no funciona".
var lineaConexion = regexp.MustCompile(
	`Player (?:connected|Spawned):\s*(.+?),?\s+xuid:\s*(\d+)`)

// pareceConexion detecta lineas que hablan de un xuid pero que no encajan.
var pareceConexion = regexp.MustCompile(`(?i)xuid`)

// ParseConnection extrae gamertag y XUID de una linea del log.
//
// El segundo valor distingue tres casos: reconocida, no tiene nada que ver, o
// parece una conexion pero no se entendio -que es el que hay que mirar-.
func ParseConnection(linea string) (Connection, Reconocimiento) {
	if m := lineaConexion.FindStringSubmatch(linea); m != nil {
		nombre := strings.TrimSpace(m[1])
		if nombre == "" {
			return Connection{}, NoEntendida
		}
		return Connection{Gamertag: nombre, XUID: m[2]}, Reconocida
	}

	if pareceConexion.MatchString(linea) {
		return Connection{}, NoEntendida
	}
	return Connection{}, Irrelevante
}

// Reconocimiento dice que se pudo sacar de una linea.
type Reconocimiento int

const (
	// Irrelevante: la linea no habla de conexiones. Es la mayoria.
	Irrelevante Reconocimiento = iota
	// Reconocida: se extrajeron gamertag y XUID.
	Reconocida
	// NoEntendida: la linea menciona un xuid pero el patron no encaja.
	// Casi siempre significa que Mojang cambio la redaccion.
	NoEntendida
)

// ScanLine cumple el puerto app.LogScanner.
//
// Traduce el resultado a booleanos sueltos para que el paquete app no tenga que
// conocer el tipo Reconocimiento, que es un detalle de como lee logs Bedrock.
func (*Flavor) ScanLine(linea string) (gamertag, xuid string, entendida, sospechosa bool) {
	c, rec := ParseConnection(linea)
	switch rec {
	case Reconocida:
		return c.Gamertag, c.XUID, true, false
	case NoEntendida:
		return "", "", false, true
	default:
		return "", "", false, false
	}
}
