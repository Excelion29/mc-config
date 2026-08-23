// Package mcworld analiza archivos de mundo de Minecraft.
//
// Es un adaptador: implementa el puerto app.WorldInspector. Todo lo que sabe de
// formatos de Minecraft (ZIP, NBT, level.dat) vive aqui y en ningun otro sitio.
package mcworld

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/nbt"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Limites de seguridad. El archivo lo sube una persona, pero su contenido viene
// de internet (D-11): se trata como no confiable.
const (
	// maxUncompressed corta las bombas zip: un archivo de pocos MB que se
	// expande a terabytes y llena el disco. Si el disco se llena, MySQL del
	// cliente deja de escribir y cae su sistema (M-2).
	maxUncompressed = 8 << 30 // 8 GiB

	// maxRatio es la otra mitad de la defensa: un mundo real comprime bien,
	// pero no 200 a 1.
	maxRatio = 200

	maxEntries = 50000
	maxIcon    = 4 << 20 // 4 MiB
)

type Inspector struct{}

func New() *Inspector { return &Inspector{} }

// Inspect valida el archivo y deduce que es.
//
// No extrae nada: solo lee la tabla del ZIP y los pocos archivos que necesita.
// La extraccion real ocurre en F3, al crear la instancia.
func (Inspector) Inspect(r io.ReaderAt, size int64) (*domain.WorldInspection, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, domain.ErrNotAnArchive
	}
	if len(zr.File) > maxEntries {
		return nil, domain.ErrZipBomb
	}

	var (
		total     int64
		levelDat  *zip.File
		levelName *zip.File
		icon      *zip.File
		hasDB     bool
		hasRegion bool
	)

	for _, f := range zr.File {
		// Zip slip: una entrada con ".." o ruta absoluta puede escribir fuera
		// del destino al extraer. En esta maquina eso incluye los archivos del
		// cliente (D-09). Se rechaza el archivo entero, no solo la entrada.
		if !safePath(f.Name) {
			return nil, fmt.Errorf("%w: %q", domain.ErrUnsafePath, f.Name)
		}

		total += int64(f.UncompressedSize64)
		if total > maxUncompressed {
			return nil, domain.ErrZipBomb
		}

		// La raiz del mundo puede estar dentro de una carpeta si el autor
		// comprimio la carpeta en vez de su contenido.
		switch strings.ToLower(path.Base(f.Name)) {
		case "level.dat":
			if levelDat == nil || depth(f.Name) < depth(levelDat.Name) {
				levelDat = f
			}
		case "levelname.txt":
			if levelName == nil || depth(f.Name) < depth(levelName.Name) {
				levelName = f
			}
		case "world_icon.jpeg", "world_icon.jpg", "icon.png":
			if icon == nil || depth(f.Name) < depth(icon.Name) {
				icon = f
			}
		}

		lower := strings.ToLower(f.Name)
		switch {
		case strings.Contains(lower, "db/"):
			hasDB = true
		case strings.Contains(lower, "region/"):
			hasRegion = true
		}
	}

	if levelDat == nil {
		return nil, domain.ErrNotAWorld
	}
	if size > 0 && total/size > maxRatio && total > 512<<20 {
		return nil, domain.ErrZipBomb
	}

	raw, err := readEntry(levelDat, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("leyendo level.dat: %w", err)
	}

	insp := &domain.WorldInspection{
		Entries:      len(zr.File),
		Uncompressed: total,
	}

	// El formato del propio level.dat es la senal mas fiable de la edicion:
	// Java lo guarda comprimido con gzip y NBT big-endian; Bedrock lo guarda
	// sin comprimir, con cabecera de 8 bytes y NBT little-endian.
	if isGzip(raw) {
		insp.Edition = domain.EditionJava
		insp.RawName, insp.Version, err = parseJavaLevel(raw)
	} else {
		insp.Edition = domain.EditionBedrock
		insp.RawName, insp.Version, err = parseBedrockLevel(raw)
	}
	if err != nil {
		return nil, err
	}

	// La estructura de carpetas solo se usa para desempatar, no para decidir.
	if insp.Edition == domain.EditionBedrock && hasRegion && !hasDB {
		insp.Edition = domain.EditionJava
	}

	// levelname.txt es texto plano y gana sobre el nombre del NBT (H-F0-4).
	if levelName != nil {
		if b, err := readEntry(levelName, 4096); err == nil {
			if n := strings.TrimSpace(string(b)); n != "" {
				insp.RawName = n
			}
		}
	}

	if icon != nil && icon.UncompressedSize64 <= maxIcon {
		if b, err := readEntry(icon, maxIcon); err == nil {
			insp.IconBytes = b
		}
	}

	return insp, nil
}

// safePath rechaza rutas absolutas y cualquier salto hacia arriba.
func safePath(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return false
	}
	// Windows: "C:algo" tambien es absoluta.
	if len(name) > 1 && name[1] == ':' {
		return false
	}
	clean := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return true
}

func depth(name string) int {
	return strings.Count(strings.ReplaceAll(name, `\`, "/"), "/")
}

func readEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, limit))
}

func isGzip(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

// parseBedrockLevel lee el level.dat de Bedrock.
//
// Formato: 4 bytes de version + 4 bytes de longitud (little-endian), y despues
// el NBT sin comprimir, tambien little-endian. La mayoria de librerias NBT
// asumen el formato de Java y devuelven basura aqui sin avisar.
func parseBedrockLevel(raw []byte) (name, version string, err error) {
	if len(raw) < 8 {
		return "", "", domain.ErrNotAWorld
	}

	declared := int64(binary.LittleEndian.Uint32(raw[4:8]))
	body := raw[8:]
	if declared > 0 && declared <= int64(len(body)) {
		body = body[:declared]
	}

	var data map[string]any
	if err := nbt.UnmarshalEncoding(body, &data, nbt.LittleEndian); err != nil {
		return "", "", fmt.Errorf("%w: level.dat de Bedrock ilegible: %v", domain.ErrNotAWorld, err)
	}

	name, _ = data["LevelName"].(string)

	if v := versionFromList(data["lastOpenedWithVersion"]); v != "" {
		version = v
	} else if v, ok := data["InventoryVersion"].(string); ok {
		version = v
	} else if v := versionFromList(data["MinimumCompatibleClientVersion"]); v != "" {
		version = v
	}

	return name, version, nil
}

// versionFromList convierte [1 21 21 3] en "1.21.21.3".
//
// Confirmado en F0 (H-F0-2): las versiones de Bedrock tienen cuatro numeros,
// mientras que las paginas de mapas publican solo los tres primeros.
//
// Se usa reflexion porque el decodificador NBT puede devolver la lista como
// []int32, []any o []int segun el caso. Con un type switch cerrado sobre []any
// devolvia cadena vacia en silencio y la version acababa saliendo de otro campo:
// un valor plausible pero equivocado, sin ningun error a la vista.
func versionFromList(v any) string {
	if v == nil {
		return ""
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return ""
	}
	if rv.Len() == 0 {
		return ""
	}

	parts := make([]string, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		n, ok := asInt(rv.Index(i))
		if !ok {
			return ""
		}
		parts = append(parts, strconv.FormatInt(n, 10))
	}

	// Se recorta el ultimo componente si es cero: "1.20.80.0" se publica
	// siempre como "1.20.80".
	for len(parts) > 3 && parts[len(parts)-1] == "0" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ".")
}

func asInt(v reflect.Value) (int64, bool) {
	// Una lista []any llega como interfaces: hay que mirar lo que contienen.
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint()), true
	}
	return 0, false
}

// parseJavaLevel lee el level.dat de Java: gzip + NBT big-endian.
func parseJavaLevel(raw []byte) (name, version string, err error) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", "", fmt.Errorf("%w: gzip ilegible: %v", domain.ErrNotAWorld, err)
	}
	defer zr.Close()

	plain, err := io.ReadAll(io.LimitReader(zr, 16<<20))
	if err != nil {
		return "", "", fmt.Errorf("%w: gzip truncado: %v", domain.ErrNotAWorld, err)
	}

	var root map[string]any
	if err := nbt.UnmarshalEncoding(plain, &root, nbt.BigEndian); err != nil {
		return "", "", fmt.Errorf("%w: level.dat de Java ilegible: %v", domain.ErrNotAWorld, err)
	}

	data, _ := root["Data"].(map[string]any)
	if data == nil {
		return "", "", domain.ErrNotAWorld
	}

	name, _ = data["LevelName"].(string)
	if v, ok := data["Version"].(map[string]any); ok {
		version, _ = v["Name"].(string)
	}
	return name, version, nil
}
