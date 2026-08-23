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
type Maps struct {
	repo      MapRepo
	store     MapStorage
	inspector MapInspector
	audit     *Audit
	clock     Clock
	log       *slog.Logger

	maxUpload int64
}

func NewMaps(repo MapRepo, store MapStorage, inspector MapInspector, audit *Audit, clock Clock, maxUpload int64, log *slog.Logger) *Maps {
	return &Maps{
		repo: repo, store: store, inspector: inspector,
		audit: audit, clock: clock, maxUpload: maxUpload, log: log,
	}
}

func (m *Maps) MaxUpload() int64 { return m.maxUpload }

func (m *Maps) List(ctx context.Context, actor *domain.User) ([]domain.Map, error) {
	if !actor.Can(domain.PermServerView) {
		return nil, domain.ErrForbidden
	}
	return m.repo.List(ctx)
}

func (m *Maps) Icon(ctx context.Context, actor *domain.User, id int64) ([]byte, error) {
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
func (m *Maps) Import(ctx context.Context, actor *domain.User, src io.Reader, fileName string, declaredSize int64, ip string) (*domain.Map, error) {
	if !actor.Can(domain.PermMapImport) {
		return nil, domain.ErrForbidden
	}
	if src == nil || strings.TrimSpace(fileName) == "" {
		return nil, domain.ErrNoFile
	}
	if declaredSize > 0 && declaredSize > m.maxUpload {
		return nil, domain.ErrFileTooBig
	}

	// Se exige margen para el archivo comprimido y para su extraccion en F3.
	// Si el disco se llena, MySQL del cliente deja de escribir (M-2).
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
		return existing, domain.ErrDuplicateMap
	} else if !errors.Is(err, domain.ErrMapNotFound) {
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

	mp := &domain.Map{
		Name:       name,
		RawName:    insp.RawName,
		Edition:    insp.Edition,
		Version:    insp.Version,
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

	m.audit.Record(ctx, actor, actor.Email, domain.ActionMapImported,
		fmt.Sprintf("%s (%s %s, %s)", mp.Name, mp.Edition.Label(), mp.Version, HumanSize(mp.SizeBytes)), ip)
	return mp, nil
}

func (m *Maps) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermMapDelete) {
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
	if err := m.store.Delete(mp.SHA256); err != nil {
		m.log.Warn("el mapa se borro de la biblioteca pero quedaron archivos",
			"sha", mp.SHA256, "error", err)
	}

	m.audit.Record(ctx, actor, actor.Email, domain.ActionMapDeleted, mp.Name, ip)
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
