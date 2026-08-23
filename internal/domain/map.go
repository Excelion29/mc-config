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

// Map es un mapa de la biblioteca: el archivo subido mas lo que se dedujo de el.
type Map struct {
	ID int64
	// Name es el nombre visible, ya limpio de codigos de formato.
	Name string
	// RawName es lo que traia el mapa. Se conserva porque los codigos de color
	// forman parte de como el autor queria que se viera dentro del juego.
	RawName    string
	Edition    Edition
	Version    string // version comercial leida de level.dat, p.ej. "1.21.21"
	FileName   string // nombre del archivo tal como se subio
	SizeBytes  int64
	SHA256     string
	HasIcon    bool
	UploadedBy int64
	CreatedAt  time.Time
}

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

// MapInspection es lo que se deduce de un archivo antes de aceptarlo.
type MapInspection struct {
	Edition   Edition
	RawName   string
	Version   string
	IconBytes []byte // world_icon.jpeg si venia incluido
	Entries   int
	Uncompressed int64
}
