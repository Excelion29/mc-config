// Package mojang resuelve identidades de Minecraft Java contra la API publica
// de Mojang.
//
// Existe porque whitelist.json y ops.json de Java identifican por UUID y no por
// nombre. En Bedrock esto no hace falta -la allow-list va por gamertag- y
// ademas seria imposible: el XUID no se conoce hasta que la persona entra
// (H-J-8). En Java si se puede preguntar de antemano, y por eso aqui NO hace
// falta el alta en dos fases.
package mojang

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

const perfilAPI = "https://api.mojang.com/users/profiles/minecraft/"

type Resolver struct {
	cliente *http.Client
}

func New() *Resolver {
	// Plazo corto: esto se consulta al dar de alta a alguien, con una persona
	// esperando delante de un formulario.
	return &Resolver{cliente: &http.Client{Timeout: 5 * time.Second}}
}

// ResolveJavaUUID traduce un nombre de Java a su UUID.
//
// Devuelve ErrJavaNameNotFound si la cuenta no existe, que es informacion util:
// casi siempre es una errata al escribir el nombre, y decirlo en ese momento
// evita el "no puedo entrar" de dentro de una semana.
func (r *Resolver) ResolveJavaUUID(ctx context.Context, nombre string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, perfilAPI+nombre, nil)
	if err != nil {
		return "", err
	}

	resp, err := r.cliente.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultando a Mojang: %w", err)
	}
	defer resp.Body.Close()

	// Mojang responde 204 sin cuerpo cuando el nombre no existe. Algunas
	// versiones devuelven 404. Los dos significan lo mismo.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return "", domain.ErrJavaNameNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Mojang respondio %d", resp.StatusCode)
	}

	var perfil struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&perfil); err != nil {
		return "", err
	}
	if perfil.ID == "" {
		return "", domain.ErrJavaNameNotFound
	}

	return ConGuiones(perfil.ID), nil
}

// ConGuiones da formato de UUID al identificador que devuelve Mojang.
//
// La API lo manda SIN guiones -"1c9bedc51bf543cbbe42931dace7be8f"- pero
// whitelist.json y ops.json lo esperan con ellos. Es la clase de detalle que no
// da error: el archivo se escribe, el servidor lo lee, y simplemente no
// reconoce a nadie.
func ConGuiones(id string) string {
	if len(id) != 32 {
		return id
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:32]
}
