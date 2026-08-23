package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// LoadFile lee un archivo .env y define las variables que falten.
//
// Lo que ya venga del entorno real MANDA sobre el archivo. Asi el mismo binario
// sirve en local (lee .env) y en Docker (recibe env_file / environment) sin
// cambiar nada, y una variable puntual se puede pisar desde la linea de
// comandos sin editar el archivo.
//
// Que el archivo no exista no es un error: en produccion no hay .env.
//
// Se implementa a mano en vez de traer godotenv: son cuarenta lineas y evita
// una dependencia mas en un binario que corre junto a la produccion de un
// cliente (D-09).
func LoadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("abriendo %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	line := 0

	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())

		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")

		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return fmt.Errorf("%s:%d: se esperaba CLAVE=valor", path, line)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("%s:%d: clave vacia", path, line)
		}

		value = unquote(strings.TrimSpace(value))

		// El entorno real gana: no se pisa lo que ya esta definido.
		if _, defined := os.LookupEnv(key); defined {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s:%d: definiendo %s: %w", path, line, key, err)
		}
	}

	if err := sc.Err(); err != nil {
		return fmt.Errorf("leyendo %s: %w", path, err)
	}
	return nil
}

// unquote retira un par de comillas envolventes. Solo cuando no las hay se
// eliminan los comentarios en linea: sin comillas un # empieza un comentario;
// con ellas forma parte del valor (una contrasena puede llevar #).
func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}
