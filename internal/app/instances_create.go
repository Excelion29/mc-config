package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Este archivo cubre la PREPARACION de un servidor: elegir version, instalar
// el mundo y dejar el contenedor definido. Encenderlo es una accion aparte y
// vive en instances_lifecycle.go, porque solo puede haber uno encendido y esa
// regla no tiene nada que ver con crearlo.

// Create prepara un servidor a partir de un mapa de la biblioteca.
//
// Instala el mundo, escribe la configuracion y precrea el contenedor, pero NO
// lo arranca: por D-02 solo puede haber uno encendido, y decidir cual es una
// accion aparte.
func (i *Instances) Create(ctx context.Context, actor *domain.User, name string, worldID int64, version string, allowlist []string, ip string) (*domain.Instance, error) {
	if !actor.Can(domain.PermInstanceCreate) {
		return nil, domain.ErrForbidden
	}

	name = strings.TrimSpace(name)
	mp, err := i.maps.ByID(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = mp.Name
	}

	slug := domain.Slugify(name)
	if slug == "" {
		return nil, domain.ErrEmptyInstanceName
	}
	if _, err := i.repo.BySlug(ctx, slug); err == nil {
		return nil, domain.ErrDuplicateInstance
	} else if !errors.Is(err, domain.ErrInstanceNotFound) {
		return nil, err
	}

	// D-01: un .mcworld no va a un servidor Java, y al reves tampoco. Se
	// comprueba aqui y no al arrancar, para no dejar una instancia creada que
	// nunca podra funcionar.
	// La version elegida tiene que ser DE LA EDICION del mundo. La pantalla
	// ofrece las dos juntas porque sin JavaScript no puede saber cual toca
	// hasta que se envia, asi que la comprobacion vive aqui.
	//
	// Sin esto, elegir una de Bedrock para un mundo de Java se traduce en una
	// descarga que falla dentro del contenedor, minutos despues, con un error
	// que no menciona la version. Se rechaza antes, diciendo por que.
	if err := i.versionValida(ctx, mp.Edition, version); err != nil {
		return nil, err
	}

	flavor, ok := i.flavors[mp.Edition]
	if !ok {
		return nil, fmt.Errorf("%w: no hay soporte para %s", domain.ErrEditionMismatch, mp.Edition.Label())
	}

	// La version del MAPA y la del SERVIDOR no son lo mismo, y confundirlas
	// rompe el arranque.
	//
	// level.dat guarda con que version se abrio el mundo por ultima vez
	// (p.ej. 1.21.21.3). Eso es un REQUISITO -"necesita 1.20.80 o superior"-,
	// no un numero de build descargable: Mojang no publica un servidor con ese
	// numero y la descarga devuelve 404. Confirmado en F0 y otra vez aqui.
	//
	// Por eso por defecto se instala LATEST, que es lo que funciono en F0.
	version = strings.TrimSpace(version)
	if version == "" {
		version = "LATEST"
	}

	inst := &domain.Instance{
		Name:      name,
		Slug:      slug,
		Edition:   mp.Edition,
		Version:   version,
		WorldID:     mp.ID,
		WorldName:   mp.Name,
		Gen:         mp.Gen,
		Rules:       mp.Rules,
		LevelName: slug,
		Port:      flavor.DefaultPort(),
		State:     domain.StateStopped,
		MemoryMB:  domain.DefaultMemoryMB,
		CPUs:      domain.DefaultCPUs,
		CreatedAt: i.clock(),
	}

	dir := i.dataDir(inst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creando %s: %w", dir, err)
	}

	// Si algo falla a partir de aqui, no debe quedar media instancia en disco.
	limpiar := func() { os.RemoveAll(dir) }

	// Un mundo creado no tiene archivo que extraer: lo genera el propio
	// servidor al arrancar, a partir de la semilla que va en la configuracion.
	// Se le pasa ruta vacia y cada adaptador decide que hacer con eso.
	archivo := ""
	if mp.Importado() {
		archivo = i.store.ArchivePath(mp.SHA256)
	}
	if err := flavor.InstallWorld(archivo, dir, inst.LevelName); err != nil {
		limpiar()
		return nil, err
	}
	// El modo, tambien al crear: WriteConfig escribe online-mode a partir de
	// el, y una instancia recien creada no habia pasado nunca por Start.
	inst.Auth = i.modoActual(ctx)
	if err := flavor.WriteConfig(inst, dir, RefsFrom(allowlist)); err != nil {
		limpiar()
		return nil, err
	}

	containerID, err := i.runtime.Create(ctx, i.specDe(ctx, flavor, inst, dir))
	if err != nil {
		limpiar()
		return nil, err
	}
	inst.ContainerID = containerID

	id, err := i.repo.Create(ctx, inst)
	if err != nil {
		i.runtime.Remove(ctx, containerID)
		limpiar()
		return nil, err
	}
	inst.ID = id

	i.audit.Record(ctx, actor, actor.Email, domain.ActionInstanceCreated,
		fmt.Sprintf("%s (%s %s) desde %s %q", inst.Name, inst.Edition.Label(),
			inst.Version, strings.ToLower(mp.Origin.Label()), mp.Name), ip)
	return inst, nil
}

// Versions lista las versiones instalables de una edicion.
func (i *Instances) Versions(ctx context.Context, actor *domain.User, edition domain.Edition) ([]VersionOption, error) {
	if !actor.Can(domain.PermInstanceCreate) {
		return nil, domain.ErrForbidden
	}
	flavor, ok := i.flavors[edition]
	if !ok {
		return nil, domain.ErrEditionMismatch
	}
	return flavor.AvailableVersions(ctx)
}

// SwitchPreview responde a "si arranco esta, a quien echo".
//
// Se separa de Start porque la confirmacion viaja por la URL tras una
// redireccion (POST/Redirect/GET) y hay que poder recomponerla en el GET.
func (i *Instances) SwitchPreview(ctx context.Context, actor *domain.User, targetID int64) (*domain.Instance, int, error) {
	if !actor.Can(domain.PermServerOperate) {
		return nil, 0, domain.ErrForbidden
	}

	running, err := i.repo.Running(ctx)
	if errors.Is(err, domain.ErrInstanceNotFound) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if running.ID == targetID {
		return nil, 0, nil
	}

	online, _, _ := i.playersOf(ctx, running)
	return running, online, nil
}

// versionValida comprueba que la version exista para esa edicion.
//
// Se acepta siempre lo vacio y LATEST -valen para las dos- y tambien una
// version que no este en la lista: Mojang y PaperMC no publican todo su
// historico, y quien sabe lo que quiere debe poder escribirlo. Lo que se
// rechaza es lo que se sabe que es de OTRA edicion, que es el error real.
func (i *Instances) versionValida(ctx context.Context, e domain.Edition, version string) error {
	if version == "" || version == "LATEST" {
		return nil
	}

	otra := domain.EditionJava
	if e == domain.EditionJava {
		otra = domain.EditionBedrock
	}

	flavor, ok := i.flavors[otra]
	if !ok {
		return nil
	}
	opciones, err := flavor.AvailableVersions(ctx)
	if err != nil {
		return nil // sin lista con que comparar, se deja pasar
	}

	for _, o := range opciones {
		if o.Value == version {
			return fmt.Errorf("%w: %s es una version de %s, y el mundo es de %s",
				domain.ErrEditionMismatch, version, otra.Label(), e.Label())
		}
	}
	return nil
}
