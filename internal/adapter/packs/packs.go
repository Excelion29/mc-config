// Package packs calcula el hash de un paquete de texturas sin guardarlo.
package packs

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxPaquete es lo mas que se descarga para hashear.
//
// Un paquete de texturas razonable pesa unos pocos MB. El tope existe porque el
// enlace es de otro: si apunta a algo enorme -o a un flujo sin fin- el panel se
// quedaria descargando por una tarea que es de comodidad, no imprescindible.
const maxPaquete = 250 << 20 // 250 MB

// Hasher descarga un paquete, lo hashea y tira los bytes.
type Hasher struct{ cliente *http.Client }

func New() *Hasher {
	return &Hasher{cliente: &http.Client{Timeout: 3 * time.Minute}}
}

// SHA1 calcula el hash que Java usa para reconocer el paquete que ya tiene.
//
// Es el unico motivo de descargar algo aqui. El archivo NO se guarda: el panel
// no aloja paquetes, solo enlaces (M-2). Se paga una descarga una vez, al
// anadirlo, para ahorrarsela a cada jugador en cada conexion.
//
// Se escribe directo al hash con io.Copy, sin pasar por memoria ni por disco:
// los bytes se leen, se suman y se tiran.
func (h *Hasher) SHA1(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := h.cliente.Do(req)
	if err != nil {
		return "", fmt.Errorf("descargando el paquete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("el enlace del paquete respondio %d", resp.StatusCode)
	}

	suma := sha1.New()
	if _, err := io.Copy(suma, io.LimitReader(resp.Body, maxPaquete)); err != nil {
		return "", fmt.Errorf("leyendo el paquete: %w", err)
	}
	return hex.EncodeToString(suma.Sum(nil)), nil
}
