package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Maps resuelve los casos de uso de la biblioteca de mapas (F2).
type Worlds struct {
	repo      WorldRepo
	store     WorldStorage
	inspector WorldInspector
	audit     *Audit
	clock     Clock
	log       *slog.Logger

	maxUpload int64
}

func NewWorlds(repo WorldRepo, store WorldStorage, inspector WorldInspector, audit *Audit, clock Clock, maxUpload int64, log *slog.Logger) *Worlds {
	return &Worlds{
		repo: repo, store: store, inspector: inspector,
		audit: audit, clock: clock, maxUpload: maxUpload, log: log,
	}
}

func (m *Worlds) MaxUpload() int64 { return m.maxUpload }

func (m *Worlds) List(ctx context.Context, actor *domain.User) ([]domain.World, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return m.repo.List(ctx)
}

// ByID devuelve un mundo suelto. Lo usa la web para repintar un trozo de
// pantalla sin recargarla entera.
func (m *Worlds) ByID(ctx context.Context, actor *domain.User, id int64) (*domain.World, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return m.repo.ByID(ctx, id)
}

func (m *Worlds) Icon(ctx context.Context, actor *domain.User, id int64) ([]byte, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	mp, err := m.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return m.store.ReadIcon(mp.SHA256)
}

// Import guarda un mapa subido por el usuario (D-11).
//
// El orden importa: primero se vuelca a un temporal con limite de tamano,
// despues se inspecciona, y solo si todo cuadra se mueve a su sitio. Asi un
// archivo malo nunca llega a la biblioteca.
func (m *Worlds) Import(ctx context.Context, actor *domain.User, src io.Reader, fileName string, declaredSize int64, ip string) (*domain.World, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}
	if src == nil || strings.TrimSpace(fileName) == "" {
		return nil, domain.ErrNoFile
	}
	if declaredSize > 0 && declaredSize > m.maxUpload {
		return nil, domain.ErrFileTooBig
	}

	// Tope por PORCENTAJE (M-2): se corta antes de llenar el disco, porque no
	// somos los unicos en esta maquina.
	if err := m.checkDisk(); err != nil {
		return nil, err
	}

	// Y ademas margen absoluto para el comprimido y su extraccion: en un disco
	// muy grande el porcentaje puede ir holgado y aun asi no caber este mapa.
	if free, err := m.store.FreeSpace(); err == nil {
		needed := uint64(m.maxUpload) * 3
		if free < needed {
			return nil, domain.ErrNoDiskSpace
		}
	}

	tmp, err := m.store.TempFile()
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	// Si algo falla, el temporal no debe quedarse ocupando disco.
	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	hasher := sha256.New()
	// LimitReader corta en maxUpload+1: si se alcanza, es que venia de mas.
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(src, m.maxUpload+1))
	if err != nil {
		return nil, fmt.Errorf("guardando la subida: %w", err)
	}
	if written > m.maxUpload {
		return nil, domain.ErrFileTooBig
	}
	if written == 0 {
		return nil, domain.ErrNoFile
	}
	if err := tmp.Sync(); err != nil {
		return nil, fmt.Errorf("sincronizando la subida: %w", err)
	}

	sha := hex.EncodeToString(hasher.Sum(nil))

	// El hash del contenido detecta el duplicado aunque el archivo se haya
	// renombrado.
	if existing, err := m.repo.BySHA(ctx, sha); err == nil {
		return existing, domain.ErrDuplicateWorld
	} else if !errors.Is(err, domain.ErrWorldNotFound) {
		return nil, err
	}

	insp, err := m.inspector.Inspect(tmp, written)
	if err != nil {
		return nil, err
	}

	name := domain.CleanName(insp.RawName)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	}

	mp := &domain.World{
		Name:    name,
		RawName: insp.RawName,
		Edition: insp.Edition,
		Version: insp.Version,
		Origin:  domain.OriginImported,
		// Un mapa importado trae su terreno hecho, asi que la generacion solo
		// describe lo que ya es. Las reglas si son nuestras.
		Gen:        domain.DefaultGeneration(),
		Rules:      domain.DefaultRules(),
		FileName:   filepath.Base(fileName),
		SizeBytes:  written,
		SHA256:     sha,
		HasIcon:    len(insp.IconBytes) > 0,
		UploadedBy: actor.ID,
		CreatedAt:  m.clock(),
	}

	// Cerrar antes de mover: en Windows no se puede renombrar un archivo
	// abierto.
	tmp.Close()

	if err := m.store.SaveArchive(sha, tmpPath); err != nil {
		return nil, err
	}
	if err := m.store.SaveIcon(sha, insp.IconBytes); err != nil {
		// La miniatura es cosmetica: que falle no invalida el mapa.
		m.log.Warn("no se pudo guardar la miniatura", "mapa", name, "error", err)
		mp.HasIcon = false
	}

	id, err := m.repo.Create(ctx, mp)
	if err != nil {
		// Si la fila no entra, el archivo huerfano se limpia: si no, ocuparia
		// disco sin aparecer en ninguna parte.
		m.store.Delete(sha)
		return nil, err
	}
	mp.ID = id

	m.audit.Record(ctx, actor, actor.Email, domain.ActionWorldImported,
		fmt.Sprintf("%s (%s %s, %s)", mp.Name, mp.Edition.Label(), mp.Version, HumanSize(mp.SizeBytes)), ip)
	return mp, nil
}

func (m *Worlds) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermWorldDelete) {
		return domain.ErrForbidden
	}

	mp, err := m.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := m.repo.Delete(ctx, id); err != nil {
		return err
	}
	// Primero la fila, despues los archivos: al reves, un fallo dejaria una
	// entrada en la biblioteca apuntando a un archivo que ya no existe.
	//
	// Y solo si HAY archivos. Un mundo creado desde una semilla no subio nada,
	// asi que no tiene hash y no hay nada que borrar (D-16). Pedirlo igual
	// reventaba el borrado -y solo el de los mundos nuevos-, despues de haber
	// quitado ya la fila.
	if mp.Importado() && mp.SHA256 != "" {
		if err := m.store.Delete(mp.SHA256); err != nil {
			m.log.Warn("el mapa se borro de la biblioteca pero quedaron archivos",
				"sha", mp.SHA256, "error", err)
		}
	}

	m.audit.Record(ctx, actor, actor.Email, domain.ActionWorldDeleted, mp.Name, ip)
	return nil
}

// HumanSize formatea bytes para mostrarlos.
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Create da de alta un mundo VACIO, que generara el servidor al arrancar.
//
// Es el camino normal para ponerse a jugar: eliges edicion, opcionalmente una
// semilla, y ya. Importar un mapa es el otro camino, y es el especial.
//
// Aqui no se toca el disco. Un mundo creado es una DECLARACION -esta semilla,
// este tipo de terreno- y los archivos no existen hasta que un servidor
// arranca con ella. Generarlos antes exigiria levantar un servidor solo para
// eso, que es justo lo que el servidor ya hace por su cuenta al encenderse.
func (m *Worlds) Create(ctx context.Context, actor *domain.User, nombre string,
	edicion domain.Edition, version, portada string, gen domain.Generation,
	reglas domain.Rules, ip string,
) (*domain.World, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}

	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return nil, domain.ErrEmptyName
	}
	if !edicion.Valid() {
		return nil, domain.ErrEditionMismatch
	}

	// Se validan aqui y no en la plantilla: un valor invalido en
	// server.properties no da error, el servidor lo ignora y se comporta de
	// otra forma sin decir nada.
	if !gen.LevelType.ValidFor(edicion) || !reglas.Gamemode.Valid() || !reglas.Difficulty.Valid() {
		return nil, domain.ErrInvalidSettings
	}
	if reglas.MaxPlayers < 1 || reglas.MaxPlayers > 100 {
		return nil, domain.ErrInvalidSettings
	}
	if !domain.PortadaValida(portada) {
		return nil, domain.ErrInvalidIconURL
	}

	// No se comprueba el disco: un mundo creado no ocupa nada hasta que se
	// enciende, y quien lo enciende es Instances, que si lo comprueba.
	mundo := &domain.World{
		Name:    nombre,
		RawName: nombre,
		Edition: edicion,
		Origin:  domain.OriginCreated,
		IconURL: strings.TrimSpace(portada),
		Gen:     gen,
		Rules:   reglas,
		// La version de un mundo CREADO significa algo distinto que la de uno
		// importado. En el importado se leyo de level.dat y dice "con esto se
		// jugo la ultima vez", que es un minimo, no una version descargable.
		// Aqui es una eleccion: con que version quieres generarlo.
		//
		// Vacia equivale a LATEST, que es lo que ya hace la creacion de
		// servidores.
		Version:    strings.TrimSpace(version),
		UploadedBy: actor.ID,
		CreatedAt:  m.clock(),
	}

	id, err := m.repo.Create(ctx, mundo)
	if err != nil {
		return nil, err
	}
	mundo.ID = id

	m.audit.Record(ctx, actor, actor.Email, domain.ActionWorldCreated,
		nombre+" ("+edicion.Label()+")", ip)
	return mundo, nil
}

// Update cambia el nombre, la portada y las reglas de un mundo.
//
// Las reglas se releen en cada arranque, asi que cambiarlas aqui basta: la
// siguiente vez que se encienda un servidor con este mundo, se aplican. La
// generacion no se toca porque no se puede: el terreno ya esta escrito.
func (m *Worlds) Update(ctx context.Context, actor *domain.User, id int64,
	nombre, portada string, reglas domain.Rules, ip string,
) (*domain.World, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}

	mundo, err := m.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}

	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return nil, domain.ErrEmptyName
	}
	if !domain.PortadaValida(portada) {
		return nil, domain.ErrInvalidIconURL
	}
	if !reglas.Gamemode.Valid() || !reglas.Difficulty.Valid() {
		return nil, domain.ErrInvalidSettings
	}
	if reglas.MaxPlayers < 1 || reglas.MaxPlayers > 100 {
		return nil, domain.ErrInvalidSettings
	}

	mundo.Name = nombre
	mundo.IconURL = strings.TrimSpace(portada)
	mundo.Rules = reglas

	if err := m.repo.Update(ctx, mundo); err != nil {
		return nil, err
	}

	m.audit.Record(ctx, actor, actor.Email, domain.ActionWorldUpdated, nombre, ip)
	return mundo, nil
}

// RulesOf da las reglas vigentes de un mundo.
//
// Lo usa Instances al arrancar, para no fiarse de la copia que hizo al crear
// la instancia: si alguien edito el mundo despues, la copia esta vieja y el
// servidor arrancaria con lo de antes sin que nada lo explique.
func (m *Worlds) RulesOf(ctx context.Context, id int64) (domain.Rules, error) {
	mundo, err := m.repo.ByID(ctx, id)
	if err != nil {
		return domain.Rules{}, err
	}
	return mundo.Rules, nil
}
