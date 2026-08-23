package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// Mojang no publica un historico de versiones del servidor de Bedrock.
//
// Este endpoint oficial devuelve solo DOS: la estable actual y la preview. No
// hay forma de listar "todas las versiones que existen", asi que la interfaz
// ofrece las que hay mas una casilla para fijar una concreta a mano.
const versionsAPI = "https://net-secondary.web.minecraft-services.net/api/v1.0/download/links"

// versionFromURL extrae "1.26.44.3" de ".../bedrock-server-1.26.44.3.zip".
var versionFromURL = regexp.MustCompile(`bedrock-server-([0-9.]+)\.zip`)

type versionCache struct {
	mu       sync.Mutex
	options  []app.VersionOption
	fetched  time.Time
	cacheTTL time.Duration
}

var cache = &versionCache{cacheTTL: 30 * time.Minute}

// AvailableVersions devuelve las versiones que se pueden instalar.
//
// Si la API no responde, se devuelven solo las palabras clave que entiende la
// imagen: es preferible una lista corta a un error que impida crear servidores.
func (*Flavor) AvailableVersions(ctx context.Context) ([]app.VersionOption, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if time.Since(cache.fetched) < cache.cacheTTL && len(cache.options) > 0 {
		return cache.options, nil
	}

	base := []app.VersionOption{
		{Value: "LATEST", Label: "Ultima estable", Recommended: true},
		{Value: "PREVIOUS", Label: "Anterior a la actual"},
	}

	stable, preview, err := fetchCurrent(ctx)
	if err != nil {
		// Sin red o con la API caida se sigue pudiendo crear servidores.
		return base, nil
	}

	opts := []app.VersionOption{
		{
			Value:       "LATEST",
			Label:       fmt.Sprintf("Ultima estable (ahora %s)", stable),
			Recommended: true,
		},
		{Value: "PREVIOUS", Label: "Anterior a la actual"},
	}
	if stable != "" {
		opts = append(opts, app.VersionOption{
			Value: stable,
			Label: stable + " — estable actual, fijada",
			Note:  "Queda clavada en esta version aunque Mojang publique otra.",
		})
	}
	if preview != "" {
		opts = append(opts, app.VersionOption{
			Value: preview,
			Label: preview + " — preview",
			Note:  "Version de pruebas de Mojang. Puede romper mapas y no se recomienda.",
		})
	}

	cache.options = opts
	cache.fetched = time.Now()
	return opts, nil
}

func fetchCurrent(ctx context.Context) (stable, preview string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsAPI, nil)
	if err != nil {
		return "", "", err
	}
	// Sin User-Agent, el servicio responde 403.
	req.Header.Set("User-Agent", "mcvps/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("consultando las versiones: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("la API de versiones devolvio %s", resp.Status)
	}

	var payload struct {
		Result struct {
			Links []struct {
				DownloadType string `json:"downloadType"`
				DownloadURL  string `json:"downloadUrl"`
			} `json:"links"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", fmt.Errorf("leyendo las versiones: %w", err)
	}

	for _, l := range payload.Result.Links {
		m := versionFromURL.FindStringSubmatch(l.DownloadURL)
		if m == nil {
			continue
		}
		switch l.DownloadType {
		case "serverBedrockLinux":
			stable = m[1]
		case "serverBedrockPreviewLinux":
			preview = m[1]
		}
	}
	return stable, preview, nil
}
