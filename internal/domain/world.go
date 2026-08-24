package domain

import (
	"strings"
	"time"
)

// Edition distingue las dos ediciones de Minecraft (D-01).
//
// No son intercambiables: un .mcworld nunca va a un servidor Java. El panel
// tiene que detectarlo e impedirlo, no dejar que falle a medias.
type Edition string

const (
	EditionBedrock Edition = "bedrock"
	EditionJava    Edition = "java"
)

func (e Edition) Valid() bool {
	return e == EditionBedrock || e == EditionJava
}

func (e Edition) Label() string {
	switch e {
	case EditionBedrock:
		return "Bedrock"
	case EditionJava:
		return "Java"
	}
	return string(e)
}

// Origin dice de donde salio un mundo.
//
// Es lo que decide que campos significan algo: un mundo importado tiene
// archivo, hash e icono; uno creado tiene semilla y tipo de terreno.
type Origin string

const (
	// OriginImported: se subio un archivo y de el se saco el mundo.
	OriginImported Origin = "imported"
	// OriginCreated: el mundo nace vacio y lo genera el servidor al arrancar
	// por primera vez, a partir de la semilla.
	//
	// No es exclusivo de Java: Bedrock tambien genera un mundo si la carpeta
	// esta vacia, cosa que se descubrio sin querer en F3.
	OriginCreated Origin = "created"
)

func (o Origin) Valid() bool { return o == OriginImported || o == OriginCreated }

func (o Origin) Label() string {
	switch o {
	case OriginImported:
		return "Mapa importado"
	case OriginCreated:
		return "Mundo nuevo"
	}
	return string(o)
}

// World es un mundo de la biblioteca.
//
// Puede venir de dos sitios: de un archivo que subiste -un mapa- o de una
// semilla, generado por el propio servidor. Lo segundo es lo normal cuando
// solo quieres jugar; lo primero es el caso especial.
type World struct {
	ID int64
	// Name es el nombre visible, ya limpio de codigos de formato.
	Name string
	// RawName es lo que traia el mapa. Se conserva porque los codigos de color
	// forman parte de como el autor queria que se viera dentro del juego.
	RawName    string
	Edition Edition
	Version string // version comercial leida de level.dat, p.ej. "1.21.21"
	Origin  Origin

	// --- Solo si Origin es OriginImported ---
	FileName  string // nombre del archivo tal como se subio
	SizeBytes int64
	SHA256    string
	HasIcon   bool

	// Gen son los ajustes con los que nacio el mundo. En uno importado
	// describen lo que traia el archivo, y en los dos casos son de solo
	// lectura una vez generado el terreno.
	Gen Generation
	// Rules son las reglas del servidor. Se releen en cada arranque, asi que
	// SI se pueden cambiar despues.
	Rules Rules

	UploadedBy int64
	CreatedAt  time.Time
}

// Importado indica si el mundo salio de un archivo subido.
func (m *World) Importado() bool { return m != nil && m.Origin == OriginImported }

// Creado indica si el mundo lo genera el servidor a partir de una semilla.
func (m *World) Creado() bool { return m != nil && m.Origin == OriginCreated }

// CleanName quita los codigos de formato de Minecraft y la decoracion.
//
// Descubierto en F0 (H-F0-4): levelname.txt del mapa de ejemplo contiene
// "§f░§e§lLucky§gBlocks§6Race§r§f░ §8v4.1". Mostrarlo tal cual llena la
// pantalla de basura.
//
// Se quitan dos cosas: los codigos "§" mas el caracter que los sigue, y los
// caracteres de bloque que los autores usan como adorno. Lo demas se conserva:
// "v4.1" es parte del nombre y sirve para distinguir versiones del mismo mapa.
func CleanName(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))

	runes := []rune(raw)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '§' {
			i++ // se salta tambien el caracter del codigo
			continue
		}
		if isDecoration(r) {
			b.WriteRune(' ') // se cambia por espacio para no pegar palabras
			continue
		}
		b.WriteRune(r)
	}

	// Los adornos dejan espacios de sobra al desaparecer.
	return strings.Join(strings.Fields(b.String()), " ")
}

func isDecoration(r rune) bool {
	switch r {
	case '░', '▒', '▓', '█', '▀', '▄', '■', '□', '●', '○', '◆', '★', '☆', '»', '«', '|':
		return true
	}
	return false
}

// WorldInspection es lo que se deduce de un archivo antes de aceptarlo.
type WorldInspection struct {
	Edition   Edition
	RawName   string
	Version   string
	IconBytes []byte // world_icon.jpeg si venia incluido
	Entries   int
	Uncompressed int64
}
