package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// migrationsDir es la ruta en el codigo fuente, relativa a la raiz del
// repositorio. Crear migraciones es una tarea de desarrollo: los archivos van
// embebidos en el binario con go:embed, asi que uno nuevo solo cuenta si existe
// al compilar. Por eso este comando exige ejecutarse dentro del repositorio.
const migrationsDir = "internal/adapter/sqlite/migrations"

var migrationName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

// newMigration crea el siguiente archivo de migracion con la plantilla de goose.
//
// La numeracion es secuencial (0001, 0002...) y no por marca de tiempo. Con un
// solo desarrollador se lee mucho mejor el orden; las marcas de tiempo solo
// compensan cuando varias personas crean migraciones a la vez y hay que evitar
// colisiones de numero.
func newMigration(name string) int {
	slug := slugify(name)
	if slug == "" {
		fmt.Fprintln(os.Stderr,
			"migracion: indica un nombre. Ejemplo:\n"+
				"  go run ./cmd/mcvps -new-migration \"instancias de servidor\"")
		return 1
	}

	if _, err := os.Stat(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr,
			"migracion: no se encontro %s\n"+
				"Ejecuta este comando desde la raiz del repositorio.\n", migrationsDir)
		return 1
	}

	next, err := nextVersion()
	if err != nil {
		fmt.Fprintln(os.Stderr, "migracion:", err)
		return 1
	}

	file := filepath.Join(migrationsDir, fmt.Sprintf("%04d_%s.sql", next, slug))
	if _, err := os.Stat(file); err == nil {
		fmt.Fprintf(os.Stderr, "migracion: %s ya existe\n", file)
		return 1
	}

	plantilla := fmt.Sprintf(`-- +goose Up
-- %s
--
-- Recuerda:
--   * SQL portable (D-14): sin funciones especificas de SQLite.
--   * Las fechas las escribe Go en RFC3339, no la base.
--   * Solo esquema y transformacion de datos existentes. Lo que necesite
--     logica de la aplicacion (hashear contrasenas, leer el catalogo de
--     permisos) va en el arranque, no aqui.


-- +goose Down
-- Deshacer lo de arriba. Si no se puede, dejarlo escrito y explicar por que.

`, name)

	if err := os.WriteFile(file, []byte(plantilla), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "migracion: no se pudo escribir:", err)
		return 1
	}

	fmt.Printf("\n  Creada  %s\n", file)
	fmt.Println("  ---------------------------------------------")
	fmt.Println("  1. Escribe el SQL en las secciones Up y Down.")
	fmt.Println("  2. Aplicala arrancando el panel, o con -seed.")
	fmt.Println("  3. Comprueba el estado con -migrations.")
	fmt.Println()
	return 0
}

// nextVersion mira los archivos existentes y devuelve el siguiente numero.
func nextVersion() (int, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return 0, fmt.Errorf("leyendo %s: %w", migrationsDir, err)
	}

	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}

// slugify convierte "Instancias de servidor" en "instancias_de_servidor".
func slugify(s string) string {
	// Los acentos se sustituyen a mano: el nombre acaba siendo un nombre de
	// archivo y conviene que sea ASCII puro.
	reemplazos := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ñ", "n", "Ü", "u",
	)
	s = reemplazos.Replace(strings.ToLower(strings.TrimSpace(s)))

	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore && b.Len() > 0:
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// listMigrations muestra que migraciones existen y cuales estan aplicadas.
func listMigrations() int {
	d, err := build(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migraciones:", err)
		return 1
	}
	defer d.close()

	aplicadas, err := d.migrations()
	if err != nil {
		fmt.Fprintln(os.Stderr, "migraciones:", err)
		return 1
	}

	archivos, _ := os.ReadDir(migrationsDir)
	nombres := map[int]string{}
	var versiones []int
	for _, e := range archivos {
		if m := migrationName.FindStringSubmatch(e.Name()); m != nil {
			n, _ := strconv.Atoi(m[1])
			nombres[n] = e.Name()
			versiones = append(versiones, n)
		}
	}
	for v := range aplicadas {
		if _, ok := nombres[v]; !ok && v != 0 {
			nombres[v] = "(archivo no encontrado)"
			versiones = append(versiones, v)
		}
	}
	sort.Ints(versiones)

	fmt.Println()
	fmt.Printf("  %-8s %-12s %s\n", "VERSION", "ESTADO", "ARCHIVO")
	fmt.Println("  ---------------------------------------------")
	for _, v := range versiones {
		estado := "pendiente"
		if aplicadas[v] {
			estado = "aplicada"
		}
		fmt.Printf("  %-8d %-12s %s\n", v, estado, nombres[v])
	}
	fmt.Println()
	return 0
}
