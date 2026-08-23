package mcworld

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/sandertv/gophertunnel/minecraft/nbt"

	"github.com/Excelion29/mc-config/internal/domain"
)

// bedrockLevelDat construye un level.dat de Bedrock: cabecera de 8 bytes
// (version + longitud, little-endian) y NBT little-endian sin comprimir.
func bedrockLevelDat(t *testing.T, name string, version []int32) []byte {
	t.Helper()

	payload, err := nbt.MarshalEncoding(map[string]any{
		"LevelName":             name,
		"lastOpenedWithVersion": version,
		"InventoryVersion":      "1.21.21",
	}, nbt.LittleEndian)
	if err != nil {
		t.Fatalf("no se pudo construir el NBT de prueba: %v", err)
	}

	out := make([]byte, 8, 8+len(payload))
	binary.LittleEndian.PutUint32(out[0:4], 8)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	return append(out, payload...)
}

// javaLevelDat construye un level.dat de Java: gzip + NBT big-endian.
func javaLevelDat(t *testing.T, name, version string) []byte {
	t.Helper()

	payload, err := nbt.MarshalEncoding(map[string]any{
		"Data": map[string]any{
			"LevelName": name,
			"Version":   map[string]any{"Name": version},
		},
	}, nbt.BigEndian)
	if err != nil {
		t.Fatalf("no se pudo construir el NBT de prueba: %v", err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(payload)
	zw.Close()
	return buf.Bytes()
}

type entry struct {
	name string
	data []byte
}

func makeZip(t *testing.T, entries []entry) ([]byte, int64) {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("creando entrada %q: %v", e.name, err)
		}
		w.Write(e.data)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("cerrando zip: %v", err)
	}
	b := buf.Bytes()
	return b, int64(len(b))
}

func TestInspectBedrock(t *testing.T) {
	// Nombre con codigos de formato, como el mapa real de F0 (H-F0-4).
	raw := "§f░§e§lLucky§gBlocks§6Race§r§f░ §8v4.1"

	data, size := makeZip(t, []entry{
		{"level.dat", bedrockLevelDat(t, "ignorado", []int32{1, 21, 21, 3})},
		{"levelname.txt", []byte(raw)},
		{"db/000001.log", []byte("datos del mundo")},
		{"world_icon.jpeg", []byte("\xff\xd8\xff\xe0falsa imagen")},
		{"behavior_packs/LuckyBlocksRace_BE/manifest.json", []byte("{}")},
	})

	insp, err := Inspector{}.Inspect(bytes.NewReader(data), size)
	if err != nil {
		t.Fatalf("no deberia fallar: %v", err)
	}

	if insp.Edition != domain.EditionBedrock {
		t.Errorf("edicion = %q, se esperaba bedrock", insp.Edition)
	}
	if insp.Version != "1.21.21.3" {
		t.Errorf("version = %q, se esperaba 1.21.21.3", insp.Version)
	}
	// levelname.txt manda sobre el LevelName del NBT.
	if insp.RawName != raw {
		t.Errorf("RawName = %q, se esperaba el de levelname.txt", insp.RawName)
	}
	if got := domain.CleanName(insp.RawName); got != "LuckyBlocksRace v4.1" {
		t.Errorf("CleanName = %q, se esperaba \"LuckyBlocksRace v4.1\"", got)
	}
	if len(insp.IconBytes) == 0 {
		t.Error("no se extrajo world_icon.jpeg")
	}
}

func TestInspectBedrockRecortaVersionCero(t *testing.T) {
	data, size := makeZip(t, []entry{
		{"level.dat", bedrockLevelDat(t, "Mundo", []int32{1, 20, 80, 0})},
		{"db/CURRENT", []byte("x")},
	})

	insp, err := Inspector{}.Inspect(bytes.NewReader(data), size)
	if err != nil {
		t.Fatalf("no deberia fallar: %v", err)
	}
	// Las paginas de mapas publican "1.20.80", no "1.20.80.0".
	if insp.Version != "1.20.80" {
		t.Errorf("version = %q, se esperaba 1.20.80", insp.Version)
	}
	if insp.RawName != "Mundo" {
		t.Errorf("RawName = %q, se esperaba Mundo", insp.RawName)
	}
}

func TestInspectJava(t *testing.T) {
	data, size := makeZip(t, []entry{
		{"level.dat", javaLevelDat(t, "Mi mundo Java", "1.20.4")},
		{"region/r.0.0.mca", []byte("chunks")},
	})

	insp, err := Inspector{}.Inspect(bytes.NewReader(data), size)
	if err != nil {
		t.Fatalf("no deberia fallar: %v", err)
	}
	if insp.Edition != domain.EditionJava {
		t.Errorf("edicion = %q, se esperaba java", insp.Edition)
	}
	if insp.Version != "1.20.4" {
		t.Errorf("version = %q, se esperaba 1.20.4", insp.Version)
	}
	if insp.RawName != "Mi mundo Java" {
		t.Errorf("RawName = %q", insp.RawName)
	}
}

func TestInspectRechazaZipSlip(t *testing.T) {
	// Esta es la razon de ser de la validacion: al extraer, esa entrada
	// escribiria fuera de la carpeta del mundo. En la VPS eso alcanza los
	// archivos del cliente (D-09).
	for _, ruta := range []string{
		"../../../etc/cron.d/puerta-trasera",
		"/etc/passwd",
		`..\..\windows\system32\algo.dll`,
	} {
		data, size := makeZip(t, []entry{
			{"level.dat", bedrockLevelDat(t, "M", []int32{1, 21, 0, 0})},
			{ruta, []byte("carga")},
		})

		_, err := Inspector{}.Inspect(bytes.NewReader(data), size)
		if !errors.Is(err, domain.ErrUnsafePath) {
			t.Errorf("ruta %q: err = %v, se esperaba ErrUnsafePath", ruta, err)
		}
	}
}

func TestInspectRechazaSinLevelDat(t *testing.T) {
	data, size := makeZip(t, []entry{
		{"leeme.txt", []byte("esto no es un mundo")},
	})

	_, err := Inspector{}.Inspect(bytes.NewReader(data), size)
	if !errors.Is(err, domain.ErrNotAWorld) {
		t.Errorf("err = %v, se esperaba ErrNotAWorld", err)
	}
}

func TestInspectRechazaNoZip(t *testing.T) {
	basura := []byte("esto no es un zip ni de lejos")
	_, err := Inspector{}.Inspect(bytes.NewReader(basura), int64(len(basura)))
	if !errors.Is(err, domain.ErrNotAnArchive) {
		t.Errorf("err = %v, se esperaba ErrNotAnArchive", err)
	}
}

func TestInspectRechazaBombaZip(t *testing.T) {
	// 1 GiB de ceros comprime a unos pocos KiB: ratio muy por encima del limite.
	grande := make([]byte, 1<<30)
	data, size := makeZip(t, []entry{
		{"level.dat", bedrockLevelDat(t, "M", []int32{1, 21, 0, 0})},
		{"relleno.bin", grande},
	})

	_, err := Inspector{}.Inspect(bytes.NewReader(data), size)
	if !errors.Is(err, domain.ErrZipBomb) {
		t.Errorf("err = %v, se esperaba ErrZipBomb", err)
	}
}

func TestCleanName(t *testing.T) {
	casos := []struct{ raw, esperado string }{
		{"§f░§e§lLucky§gBlocks§6Race§r§f░ §8v4.1", "LuckyBlocksRace v4.1"},
		{"Mundo normal", "Mundo normal"},
		{"§aVerde§r", "Verde"},
		{"", ""},
		{"§§§", ""},
	}
	for _, c := range casos {
		if got := domain.CleanName(c.raw); got != c.esperado {
			t.Errorf("CleanName(%q) = %q, se esperaba %q", c.raw, got, c.esperado)
		}
	}
}

func TestSafePath(t *testing.T) {
	seguras := []string{"level.dat", "db/000001.log", "behavior_packs/x/manifest.json"}
	for _, p := range seguras {
		if !safePath(p) {
			t.Errorf("safePath(%q) = false, deberia ser segura", p)
		}
	}

	inseguras := []string{"../fuera", "/absoluta", `C:\windows`, "a/../../b", ""}
	for _, p := range inseguras {
		if safePath(p) {
			t.Errorf("safePath(%q) = true, deberia rechazarse", p)
		}
	}
}

func TestVersionFromList(t *testing.T) {
	if got := versionFromList([]any{int32(1), int32(21), int32(21), int32(3)}); got != "1.21.21.3" {
		t.Errorf("got %q", got)
	}
	if got := versionFromList([]any{int32(1), int32(20), int32(80), int32(0)}); got != "1.20.80" {
		t.Errorf("got %q", got)
	}
	if got := versionFromList("no es una lista"); got != "" {
		t.Errorf("got %q, se esperaba cadena vacia", got)
	}
}

func TestDepthPrefiereRaiz(t *testing.T) {
	// Si el autor comprimio la carpeta en vez de su contenido, hay dos
	// level.dat posibles: debe ganar el menos profundo.
	if depth("level.dat") >= depth("mundo/level.dat") {
		t.Error("depth deberia ser menor en la raiz")
	}
	if !strings.Contains("mundo/level.dat", "/") {
		t.Error("comprobacion de apoyo")
	}
}
