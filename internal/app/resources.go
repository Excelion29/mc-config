package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// ResourceRepo guarda la biblioteca de recursos y a que mundo va cada uno.
type ResourceRepo interface {
	List(ctx context.Context, kind domain.ResourceKind) ([]domain.Resource, error)
	ByID(ctx context.Context, id int64) (*domain.Resource, error)
	Create(ctx context.Context, r *domain.Resource) (int64, error)
	Update(ctx context.Context, r *domain.Resource) error
	Delete(ctx context.Context, id int64) error
	PorURL(ctx context.Context, url string) (*domain.Resource, error)

	DeMundo(ctx context.Context, worldID int64) ([]domain.ResourceAsignado, error)
	Asignar(ctx context.Context, worldID int64, ids []int64, principal int64) error
	PrincipalDe(ctx context.Context, worldID int64) (domain.PackRef, error)
	SetRequired(ctx context.Context, worldID int64, requerido bool) error
}

// Inspeccion es lo que el panel averigua de un enlace mirandolo una vez.
type Inspeccion struct {
	// Probado dice si se llego a abrir el enlace. Sin esto no se distingue "es
	// una pagina" de "no respondio".
	Probado bool
	// Directo dice si lo que devolvio era el ARCHIVO y no una pagina.
	Directo bool
	// SHA1 solo sale cuando el enlace da el archivo.
	SHA1 string
	// Titulo es el nombre que la propia pagina se da. Sirve para no obligar a
	// nadie a bautizar un enlace que acaba de pegar.
	Titulo string
}

// ResourceProbe mira que hay al otro lado de un enlace.
//
// Es un puerto porque salir a la red es infraestructura. Existe para que el
// unico campo obligatorio sea el enlace: el nombre y la huella los averigua el
// panel, que para eso esta.
type ResourceProbe interface {
	Inspeccionar(ctx context.Context, url string) (Inspeccion, error)
}

// Resources es la biblioteca de recursos.
type Resources struct {
	repo  ResourceRepo
	probe ResourceProbe
	audit *Audit
	clock Clock
	log   *slog.Logger
}

func NewResources(repo ResourceRepo, probe ResourceProbe, audit *Audit, clock Clock, log *slog.Logger) *Resources {
	return &Resources{repo: repo, probe: probe, audit: audit, clock: clock, log: log}
}

// List es la unica operacion que NO exige poder gestionar recursos.
//
// Es la que usa quien solo juega: entra al panel, ve que necesita el mapa y
// pincha el enlace para instalarselo. Por eso mirar y tocar son permisos
// distintos.
func (s *Resources) List(ctx context.Context, actor *domain.User, kind domain.ResourceKind) ([]domain.Resource, error) {
	if !actor.Can(domain.PermResourceView) {
		return nil, domain.ErrForbidden
	}
	return s.repo.List(ctx, kind)
}

// Create anade un recurso a la biblioteca.
//
// Lo unico obligatorio es el enlace. El nombre es una mascara: si va vacio, el
// panel intenta sacar el titulo de la propia pagina, y si tampoco puede se
// ensena el enlace tal cual. Obligar a bautizarlo seria pedir trabajo por algo
// que casi siempre se puede deducir.
func (s *Resources) Create(ctx context.Context, actor *domain.User,
	kind domain.ResourceKind, url, nombre, nota, ip string) (*domain.Resource, error) {

	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}
	if !kind.Valid() {
		kind = domain.KindTexturePack
	}

	url = strings.TrimSpace(url)
	if !domain.RecursoURLValida(url) {
		return nil, domain.ErrResourceURLInvalida
	}

	r := &domain.Resource{
		Kind:      kind,
		URL:       url,
		Name:      strings.TrimSpace(nombre),
		Note:      strings.TrimSpace(nota),
		CreatedBy: actor.ID,
		CreatedAt: s.clock(),
	}
	s.completar(ctx, r)

	id, err := s.repo.Create(ctx, r)
	if err != nil {
		return nil, err
	}
	r.ID = id

	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceCreated, r.Etiqueta(), ip)
	return r, nil
}

// Update cambia el nombre, el enlace o la nota.
func (s *Resources) Update(ctx context.Context, actor *domain.User, id int64,
	url, nombre, nota, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}

	url = strings.TrimSpace(url)
	if !domain.RecursoURLValida(url) {
		return domain.ErrResourceURLInvalida
	}

	// Solo se vuelve a mirar el enlace si CAMBIO. Descargar otra vez el archivo
	// por corregir una falta en la nota seria pagar una descarga por nada.
	cambio := url != r.URL
	r.URL, r.Name, r.Note = url, strings.TrimSpace(nombre), strings.TrimSpace(nota)
	if cambio {
		r.SHA1, r.AutoName = "", ""
		r.Probado, r.Directo = false, false
		s.completar(ctx, r)
	}

	if err := s.repo.Update(ctx, r); err != nil {
		return err
	}

	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceUpdated, r.Etiqueta(), ip)
	return nil
}

func (s *Resources) Delete(ctx context.Context, actor *domain.User, id int64, ip string) error {
	if !actor.Can(domain.PermWorldDelete) {
		return domain.ErrForbidden
	}

	r, err := s.repo.ByID(ctx, id)
	if err != nil {
		return err
	}
	// Las asignaciones se van en cascada con el: un recurso borrado no puede
	// seguir figurando en la lista de ningun mundo.
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceDeleted, r.Etiqueta(), ip)
	return nil
}

func (s *Resources) DeMundo(ctx context.Context, actor *domain.User, worldID int64) ([]domain.ResourceAsignado, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}
	return s.repo.DeMundo(ctx, worldID)
}

// AnadirAMundo crea el recurso y lo asigna al mundo de una vez.
//
// Es el camino corto, y el que se usa casi siempre: pegas un enlace en la ficha
// del mapa y ya esta. Antes eran cinco pasos -ir a la biblioteca, anadir,
// volver, abrir el dialogo, marcar y guardar- para lo que en la cabeza de quien
// lo hace es uno solo.
//
// Si el enlace YA esta en la biblioteca no se duplica: se reutiliza.
func (s *Resources) AnadirAMundo(ctx context.Context, actor *domain.User, worldID int64,
	kind domain.ResourceKind, url, nombre, nota, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	url = strings.TrimSpace(url)
	if !domain.RecursoURLValida(url) {
		return domain.ErrResourceURLInvalida
	}

	existente, err := s.repo.PorURL(ctx, url)
	if err != nil && err != domain.ErrResourceNotFound {
		return err
	}

	if existente == nil {
		existente, err = s.Create(ctx, actor, kind, url, nombre, nota, ip)
		if err != nil {
			return err
		}
	}
	return s.engancharse(ctx, actor, worldID, existente, ip)
}

// EngancharAMundo asigna un recurso que YA esta en la biblioteca.
//
// Es lo que hace que compartirlos entre mapas valga de algo. Sin esto habria
// que ir a buscar su enlace y volver a pegarlo, que no es reutilizar: es
// repetir.
func (s *Resources) EngancharAMundo(ctx context.Context, actor *domain.User,
	worldID, resourceID int64, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	recurso, err := s.repo.ByID(ctx, resourceID)
	if err != nil {
		return err
	}
	return s.engancharse(ctx, actor, worldID, recurso, ip)
}

// engancharse anade un recurso a la lista de un mundo, respetando el tope.
//
// Lo comparten las dos formas de anadir -pegar un enlace y elegir uno de la
// biblioteca- porque las reglas son las mismas y tenerlas en dos sitios acaba
// con dos comportamientos distintos para la misma accion.
func (s *Resources) engancharse(ctx context.Context, actor *domain.User,
	worldID int64, recurso *domain.Resource, ip string) error {

	actuales, err := s.repo.DeMundo(ctx, worldID)
	if err != nil {
		return err
	}

	ids := []int64{recurso.ID}
	var principal int64
	for _, a := range actuales {
		if a.ID == recurso.ID {
			return domain.ErrResourceYaEnMundo
		}
		ids = append(ids, a.ID)
		if a.Principal {
			principal = a.ID
		}
	}

	if len(actuales) >= domain.MaxRecursosPorMundo {
		return domain.ErrDemasiadosRecursos
	}

	// El primero que sabe aplicarse solo se queda de principal, si no habia
	// ninguno. Es lo que la gente espera al anadir su primer paquete, y dejarlo
	// sin marcar significaria que no pasa nada al entrar al servidor.
	if principal == 0 && recurso.Automatico() {
		principal = recurso.ID
	}

	if err := s.repo.Asignar(ctx, worldID, ids, principal); err != nil {
		return err
	}

	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceAssigned, recurso.Etiqueta(), ip)
	return nil
}

// Disponibles son los de la biblioteca que un mundo NO lleva todavia.
//
// Se calcula aqui y no en la plantilla porque "cuales me faltan" es una resta
// entre dos listas, y una plantilla que resta acaba equivocandose en silencio.
func (s *Resources) Disponibles(ctx context.Context, actor *domain.User, worldID int64) ([]domain.Resource, error) {
	if !actor.Can(domain.PermWorldImport) {
		return nil, domain.ErrForbidden
	}

	todos, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	puestos, err := s.repo.DeMundo(ctx, worldID)
	if err != nil {
		return nil, err
	}

	ya := make(map[int64]bool, len(puestos))
	for _, p := range puestos {
		ya[p.ID] = true
	}

	var out []domain.Resource
	for _, r := range todos {
		if !ya[r.ID] {
			out = append(out, r)
		}
	}
	return out, nil
}

// QuitarDeMundo saca un recurso de un mundo, sin borrarlo de la biblioteca.
func (s *Resources) QuitarDeMundo(ctx context.Context, actor *domain.User,
	worldID, resourceID int64, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	actuales, err := s.repo.DeMundo(ctx, worldID)
	if err != nil {
		return err
	}

	var (
		ids       []int64
		principal int64
		quitado   string
	)
	for _, a := range actuales {
		if a.ID == resourceID {
			quitado = a.Etiqueta()
			continue
		}
		ids = append(ids, a.ID)
		if a.Principal {
			principal = a.ID
		}
	}
	if quitado == "" {
		return domain.ErrResourceNotFound
	}

	if err := s.repo.Asignar(ctx, worldID, ids, principal); err != nil {
		return err
	}

	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceAssigned, quitado, ip)
	return nil
}

// HacerPrincipal elige cual se aplica solo. Con 0, ninguno.
//
// Es una accion suya y no un campo dentro de un formulario grande: cambiar cual
// se aplica es lo que de verdad hace algo aqui, y merece su propio boton y su
// propia linea en el registro.
func (s *Resources) HacerPrincipal(ctx context.Context, actor *domain.User,
	worldID, resourceID int64, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}

	actuales, err := s.repo.DeMundo(ctx, worldID)
	if err != nil {
		return err
	}

	var (
		ids     []int64
		elegido *domain.ResourceAsignado
	)
	for i := range actuales {
		ids = append(ids, actuales[i].ID)
		if actuales[i].ID == resourceID {
			elegido = &actuales[i]
		}
	}

	if resourceID != 0 {
		if elegido == nil {
			return domain.ErrResourcePrincipalNoAsignado
		}
		// Un enlace que devuelve una pagina no se puede aplicar solo: el
		// cliente pediria el archivo y recibiria HTML. Se rechaza al elegirlo y
		// no al arrancar, porque aqui todavia hay una pantalla donde decirlo.
		if !elegido.Automatico() {
			return domain.ErrResourceNoAutomatico
		}
	}

	if err := s.repo.Asignar(ctx, worldID, ids, resourceID); err != nil {
		return err
	}
	// Sin ninguno que se aplique solo, exigirlo no significa nada.
	if resourceID == 0 {
		if err := s.repo.SetRequired(ctx, worldID, false); err != nil {
			return err
		}
	}

	nombre := "ninguno"
	if elegido != nil {
		nombre = elegido.Etiqueta()
	}
	s.audit.Record(ctx, actor, actor.Email, domain.ActionResourceAssigned, nombre, ip)
	return nil
}

// SetRequerido decide si se echa a quien rechace el recurso principal.
func (s *Resources) SetRequerido(ctx context.Context, actor *domain.User,
	worldID int64, requerido bool, ip string) error {

	if !actor.Can(domain.PermWorldImport) {
		return domain.ErrForbidden
	}
	return s.repo.SetRequired(ctx, worldID, requerido)
}

// PrincipalDe es lo que lee la instancia al arrancar.
func (s *Resources) PrincipalDe(ctx context.Context, worldID int64) (domain.PackRef, error) {
	return s.repo.PrincipalDe(ctx, worldID)
}

// completar rellena lo que se puede deducir del enlace: titulo y huella.
//
// Es de cortesia entera. Si falla, el recurso se guarda igual: sin huella el
// cliente se lo descarga en cada conexion, y sin titulo se ensena el enlace.
// Bloquear el alta porque un servidor ajeno no responde seria dejar de hacer lo
// unico que el panel si controla.
func (s *Resources) completar(ctx context.Context, r *domain.Resource) {
	// Un nombre sacado del propio enlace, sin salir a la red. Es el peor de los
	// tres, y por eso se pone primero: lo pisa el titulo de la pagina si se
	// consigue leer.
	if r.AutoName == "" {
		r.AutoName = domain.TituloDeEnlace(r.URL)
	}

	if s.probe == nil {
		return
	}

	info, err := s.probe.Inspeccionar(ctx, r.URL)
	if err != nil {
		s.log.Warn("no se pudo inspeccionar el enlace del recurso",
			"url", r.URL, "error", err)
		return
	}

	r.Probado, r.Directo = info.Probado, info.Directo
	if info.SHA1 != "" {
		r.SHA1 = info.SHA1
	}
	// El titulo de la pagina manda sobre el nombre del archivo: es lo que el
	// autor decidio llamarlo.
	if t := strings.TrimSpace(info.Titulo); t != "" {
		r.AutoName = t
	}
}
