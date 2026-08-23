package bedrock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Excelion29/mc-config/internal/app"
)

func TestParseConnection(t *testing.T) {
	casos := []struct {
		nombre   string
		linea    string
		quiero   Connection
		esperado Reconocimiento
	}{
		{
			"formato con coma",
			"[2026-08-23 11:02:15:334 INFO] Player connected: Wronkow29, xuid: 2535413418839840",
			Connection{Gamertag: "Wronkow29", XUID: "2535413418839840"},
			Reconocida,
		},
		{
			"formato Spawned, sin coma antes de xuid",
			"[2026-08-23 11:02:16:001 INFO] Player Spawned: Wronkow29 xuid: 2535413418839840, pfid: a1b2c3",
			Connection{Gamertag: "Wronkow29", XUID: "2535413418839840"},
			Reconocida,
		},
		{
			// Los gamertags de Xbox admiten espacios. Cortar por el primero
			// dejaria fuera a media gente.
			"gamertag con espacios",
			"[INFO] Player connected: El Duque 77, xuid: 111222333",
			Connection{Gamertag: "El Duque 77", XUID: "111222333"},
			Reconocida,
		},
		{
			"desconexion: tambien trae xuid y tambien vale",
			"[INFO] Player disconnected: Wronkow29, xuid: 2535413418839840",
			Connection{},
			NoEntendida,
		},
		{
			"linea normal del arranque",
			"[2026-08-23 11:00:02:118 INFO] Server started.",
			Connection{},
			Irrelevante,
		},
		{
			"linea de un mundo cargando",
			"[INFO] Level Name: LuckyBlocksRace v4.1",
			Connection{},
			Irrelevante,
		},
		{
			// Si Mojang cambia la redaccion, esto es lo que queremos: que se
			// note. Una linea con xuid que no entendemos es un aviso, no un
			// silencio.
			"formato futuro desconocido",
			"[INFO] Player joined the game (xuid=2535413418839840)",
			Connection{},
			NoEntendida,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got, rec := ParseConnection(c.linea)
			if rec != c.esperado {
				t.Fatalf("reconocimiento = %v, se esperaba %v", rec, c.esperado)
			}
			if got != c.quiero {
				t.Errorf("conexion = %+v, se esperaba %+v", got, c.quiero)
			}
		})
	}
}

func TestWritePermissions(t *testing.T) {
	dir := t.TempDir()

	if err := WritePermissions(dir, []app.OpEntry{
		{XUID: "2535413418839840", Gamertag: "Wronkow29"},
		// Sin XUID: dado de alta pero nunca ha entrado. No puede ser op
		// todavia, y colarlo con el gamertag corromperia el archivo.
		{XUID: "", Gamertag: "AmigoNuevo"},
		{XUID: "  ", Gamertag: "SoloEspacios"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "permissions.json"))
	if err != nil {
		t.Fatal(err)
	}

	quiero := `[{"permission":"operator","xuid":"2535413418839840"}]`
	if string(data) != quiero {
		t.Errorf("permissions.json =\n  %s\nse esperaba\n  %s", data, quiero)
	}
}

// TestWritePermissionsVacio: sin ops se escribe una lista vacia, no se omite el
// archivo. Un archivo ausente y uno vacio valen lo mismo para el servidor, pero
// solo el segundo deja claro que el panel lo gestiona.
func TestWritePermissionsVacio(t *testing.T) {
	dir := t.TempDir()
	if err := WritePermissions(dir, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "permissions.json"))
	if err != nil {
		t.Fatalf("no se escribio el archivo: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("permissions.json = %s, se esperaba []", data)
	}
}
