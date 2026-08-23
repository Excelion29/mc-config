// Package java implementa app.ServerFlavor para Minecraft Java con Paper.
//
// Se eligio Paper y no Forge ni Fabric (D-15): la razon de querer Java eran los
// amigos no premium, y eso pasa por AuthMe, que es un plugin de Paper. Los mods
// cierran esa puerta.
//
// Todo lo que hay aqui sale de la validacion manual de 13-validacion-java.md.
// No se dio por buena ninguna suposicion: el formato de server.properties, el
// de whitelist.json y el de ops.json se leyeron de un servidor real.
package java

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// escapePropertyValue escapa un valor para un archivo .properties de Java.
//
// Los .properties NO son un formato de "clave=valor" cualquiera: los dos
// puntos y el signo igual son separadores, y dentro de un valor van escapados.
// El servidor real escribe:
//
//	level-type=minecraft\:normal
//
// Escribirlo sin la barra deja el archivo mal (H-J-11). Es la clase de detalle
// que no da error: el servidor arranca y algo no va, sin decir que.
//
// Tambien se escapan las barras invertidas -si no, escapar lo demas no
// significaria nada- y los saltos de linea, que partirian el archivo en dos.
func escapePropertyValue(v string) string {
	var b strings.Builder
	b.Grow(len(v) + 8)
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case ':':
			b.WriteString(`\:`)
		case '=':
			b.WriteString(`\=`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// writeProperties escribe un server.properties ordenado alfabeticamente.
//
// El orden fijo no es cosmetico: hace que dos generaciones seguidas produzcan
// el mismo archivo, asi que un `diff` sobre el disco de la VPS ensena lo que
// cambio de verdad y no el capricho del recorrido de un mapa.
func writeProperties(dataDir string, props map[string]string) error {
	claves := make([]string, 0, len(props))
	for k := range props {
		claves = append(claves, k)
	}
	sort.Strings(claves)

	var b strings.Builder
	b.WriteString("# Generado por MCVPS. Los cambios a mano se pierden.\n")
	for _, k := range claves {
		fmt.Fprintf(&b, "%s=%s\n", k, escapePropertyValue(props[k]))
	}

	ruta := filepath.Join(dataDir, "server.properties")
	if err := os.WriteFile(ruta, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("escribiendo server.properties: %w", err)
	}
	return nil
}
