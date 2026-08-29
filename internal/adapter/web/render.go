package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// PageData es lo que recibe toda plantilla. Las paginas concretas anaden lo
// suyo embebiendo esta estructura.
type PageData struct {
	Title string
	User  *domain.User
	Error string
	Info  string
	Email string
	// Disco se rellena solo cuando hay algo que decir. Va en PageData y no en
	// la pantalla de mapas porque un disco lleno tumba el panel entero -y de
	// paso lo del cliente que comparte la VPS-, asi que no puede depender de
	// que alguien pase por la pantalla correcta.
	Disco *app.DiskStatus
	// Refresh, si no es cero, recarga la pagina cada N segundos. Se usa
	// mientras algo esta en transicion: sin JavaScript, es la unica forma de
	// que el estado se actualice solo (la CSP prohibe scripts en linea).
	Refresh int
}

// paginador es lo que consume la plantilla compartida del mismo nombre.
//
// Base es la URL hasta el parametro de pagina, ya con los filtros dentro y
// terminada en "?" o "&". Se arma en el handler y no en el HTML porque
// concatenar URLs con condicionales dentro de una plantilla es justo como se
// pierde un filtro al pasar de pagina.
type paginador struct {
	Info app.PageInfo
	Base string
}

type accessPageData struct {
	PageData
	Estado app.Estado
}

type auditPageData struct {
	PageData
	Page    app.AuditPage
	Actions []domain.Action
	Pag     paginador
}

type usersPageData struct {
	PageData
	// Users son filas de vista, no domain.User: ver views.go.
	Users []userRow
	// Roles se queda como tipo de dominio porque aqui solo alimenta un
	// desplegable de id y nombre; no se decide nada con el.
	Roles []domain.Role
	Pag   paginador
}

type worldsPageData struct {
	PageData
	Maps      []domain.World
	MaxUpload string
	// Las versiones de las DOS ediciones viajan juntas porque el formulario
	// ofrece las dos listas y CSS ensena la que toque. Sin JavaScript no se
	// puede repoblar un desplegable al cambiar de edicion, asi que se mandan
	// ambas y se oculta una.
	VersionsBedrock []grupoVersiones
	VersionsJava    []grupoVersiones
	// Los tipos de terreno tambien difieren por edicion.
	TypesBedrock []domain.LevelType
	TypesJava    []domain.LevelType
	Pag             paginador
}

type instancesPageData struct {
	PageData
	EnTransicion bool
	Instances    []domain.Instance
	// Current es la instancia encendida, o la que esta arrancando o parando.
	// Solo puede haber una (D-02), asi que no es una fila mas de la tabla:
	// es LA informacion de la pantalla y se presenta aparte.
	Current *domain.Instance
	Maps      []domain.World
	Online    int
	MaxOnline int
	Versions  []grupoVersiones
	Confirm   *confirmSwitch
}

type playersPageData struct {
	PageData
	Players []domain.Player
	Filtro  app.PlayerFilter
	Estados [][2]string
	Pag     paginador
	// Modo decide que identidad vale en Java, y por tanto quien puede jugar.
	// Sin el, la pantalla diria que un amigo sin cuenta comprada no puede
	// entrar justo cuando si puede.
	Modo domain.AuthMode
}

type rolesPageData struct {
	PageData
	Roles []roleView
	// Permisos da, por rol, el conjunto marcado. Lo necesita el dialogo de
	// edicion y no la tabla, por eso viaja aparte de roleView.
	Permisos map[int64]map[domain.Permission]bool
	Groups   []permissionGroup
}

// funcs son las pocas ayudas que necesitan las plantillas. Se mantiene corto a
// proposito: la logica pertenece a los casos de uso, no al HTML.
var funcs = template.FuncMap{
	"sub": func(a, b int) int { return a - b },
}

// renderer cachea una plantilla por pagina, cada una compuesta con el layout.
// Se compilan al arrancar: un error de sintaxis debe reventar en el arranque,
// no en la cara del usuario a mitad de una peticion.
type renderer struct {
	pages map[string]*template.Template
}

func newRenderer(fsys fs.FS) (*renderer, error) {
	names, err := fs.Glob(fsys, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("buscando plantillas: %w", err)
	}

	r := &renderer{pages: make(map[string]*template.Template)}
	for _, path := range names {
		base := path[len("templates/"):]
		if base == "layout.html" {
			continue
		}
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(fsys, "templates/layout.html", path)
		if err != nil {
			return nil, fmt.Errorf("compilando %s: %w", base, err)
		}
		r.pages[base] = t
	}

	if len(r.pages) == 0 {
		return nil, fmt.Errorf("no se encontro ninguna plantilla")
	}
	return r, nil
}

// pagina construye la PageData comun.
//
// Existe para que un dato que sale en TODAS las pantallas -el aviso de disco-
// se resuelva en un sitio. Cuando cada handler la armaba a mano, anadir un
// campo compartido significaba acordarse de siete sitios, y basta con olvidar
// uno para que el aviso no aparezca justo en la pantalla desde la que alguien
// esta llenando el disco.
func (s *Server) pagina(r *http.Request, titulo, errMsg, infoMsg string) PageData {
	pd := PageData{
		Title: titulo,
		User:  userFrom(r),
		Error: errMsg,
		Info:  infoMsg,
	}

	// Solo se rellena si hay algo que decir: por debajo del umbral, ni se
	// menciona. Un aviso permanente deja de leerse.
	if estado, err := s.worlds.Disk(); err == nil && estado.Avisar() {
		pd.Disco = &estado
	}
	return pd
}

// fragment escribe UNA plantilla suelta en vez de la pagina entera.
//
// Es la mitad servidor del intercambio de filas: el navegador pide una accion
// y recibe de vuelta solo la fila afectada. La plantilla es LA MISMA que usa
// el listado completo, asi que una fila recargada y una fila recién pintada no
// pueden divergir; si se escribieran por separado, acabarian haciendolo.
func (r *renderer) fragment(w http.ResponseWriter, page, name string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("plantilla desconocida: %s", page)
	}

	// A memoria primero, igual que render: si la plantilla falla a mitad no
	// se puede retirar lo ya enviado, y el navegador insertaria medio HTML.
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("renderizando el fragmento %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := buf.WriteTo(w)
	return err
}

// render escribe la pagina. Se renderiza primero a memoria para no enviar una
// respuesta a medias si la plantilla falla: con un ResponseWriter directo ya se
// habrian mandado la cabecera y medio HTML.
func (r *renderer) render(w http.ResponseWriter, status int, page string, data any) {
	t, ok := r.pages[page]
	if !ok {
		http.Error(w, "plantilla desconocida", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, "error al generar la pagina", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}
