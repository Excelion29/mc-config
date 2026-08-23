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

type auditPageData struct {
	PageData
	Page    app.AuditPage
	Actions []domain.Action
	Pag     paginador
}

type usersPageData struct {
	PageData
	Users []domain.User
	Roles []domain.Role
	Pag   paginador
}

type mapsPageData struct {
	PageData
	Maps      []domain.Map
	MaxUpload string
	Pag       paginador
}

type instancesPageData struct {
	PageData
	EnTransicion bool
	Instances    []domain.Instance
	// Current es la instancia encendida, o la que esta arrancando o parando.
	// Solo puede haber una (D-02), asi que no es una fila mas de la tabla:
	// es LA informacion de la pantalla y se presenta aparte.
	Current *domain.Instance
	Maps      []domain.Map
	Online    int
	MaxOnline int
	Versions  []app.VersionOption
	Confirm   *confirmSwitch
}

type playersPageData struct {
	PageData
	Players []domain.Player
	Pag     paginador
}

type rolesPageData struct {
	PageData
	Roles  []domain.Role
	Groups []permissionGroup
	// UsersByRole permite avisar de cuanta gente quedaria sin rol al borrarlo.
	UsersByRole map[int64]int
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
