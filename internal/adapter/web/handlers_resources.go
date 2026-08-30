package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/domain"
)

const rutaRecursos = "/resources"

func (s *Server) showResources(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	info, errMsg := s.takeFlash(w, r)

	lista, err := s.recursos.List(r.Context(), actor, "")
	if err != nil {
		s.renderFailure(w, actor, "Recursos", "No se pudo leer la biblioteca.", err)
		return
	}

	s.renderer.render(w, http.StatusOK, "resources.html", resourcesPageData{
		PageData:  s.pagina(r, "Recursos", errMsg, info),
		Resources: vistasDeRecurso(lista),
	})
}

func (s *Server) createResource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaRecursos, "No se pudo leer el formulario.")
		return
	}

	_, err := s.recursos.Create(r.Context(), userFrom(r), domain.KindTexturePack,
		r.PostFormValue("url"), r.PostFormValue("name"), r.PostFormValue("note"), clientIP(r))
	if err != nil {
		s.redirectError(w, r, rutaRecursos, s.resourceError(err))
		return
	}
	s.redirectInfo(w, r, rutaRecursos, "Recurso anadido.")
}

func (s *Server) updateResource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaRecursos, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaRecursos, "Recurso invalido.")
		return
	}

	if err := s.recursos.Update(r.Context(), userFrom(r), id,
		r.PostFormValue("url"), r.PostFormValue("name"), r.PostFormValue("note"), clientIP(r)); err != nil {
		s.redirectError(w, r, rutaRecursos, s.resourceError(err))
		return
	}
	s.redirectInfo(w, r, rutaRecursos, "Recurso actualizado.")
}

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaRecursos, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaRecursos, "Recurso invalido.")
		return
	}

	if err := s.recursos.Delete(r.Context(), userFrom(r), id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaRecursos, s.resourceError(err))
		return
	}
	s.redirectInfo(w, r, rutaRecursos,
		"Recurso borrado. Los mundos que lo llevaban se quedan sin el.")
}

// addResourceToWorld es el camino corto: pegar un enlace en la ficha del mundo.
//
// Crea el recurso si hace falta y lo asigna de una vez. Es lo que se usa casi
// siempre, porque quien esta mirando un mapa concreto no quiere darse una vuelta
// por la biblioteca para volver al mismo sitio.
func (s *Server) addResourceToWorld(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	worldID, err := strconv.ParseInt(r.PostFormValue("world_id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/worlds", "Mundo invalido.")
		return
	}

	// Dos formas de anadir por el mismo sitio: pegar un enlace nuevo, o elegir
	// uno que ya esta en la biblioteca. Se distinguen por el campo que llega.
	var err2 error
	if id, err := strconv.ParseInt(r.PostFormValue("resource_id"), 10, 64); err == nil && id > 0 {
		err2 = s.recursos.EngancharAMundo(r.Context(), userFrom(r), worldID, id, clientIP(r))
	} else {
		err2 = s.recursos.AnadirAMundo(r.Context(), userFrom(r), worldID,
			domain.KindTexturePack, r.PostFormValue("url"), r.PostFormValue("name"),
			r.PostFormValue("note"), clientIP(r))
	}
	if err2 != nil {
		s.redirectError(w, r, "/worlds", s.resourceError(err2))
		return
	}
	s.responderRecursos(w, r, worldID,
		"Recurso anadido al mundo. Se aplica en el proximo arranque del servidor.")
}

// worldResourceAction resuelve las tres acciones sobre un recurso de un mundo.
//
// Cada una es un boton suyo en la fila, como en el resto de tablas del panel.
// Estuvo todo dentro de un formulario grande con casillas y radios, y era
// ilegible: dos controles distintos por fila y una fila fantasma para "ninguno".
func (s *Server) worldResourceAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	worldID, err := strconv.ParseInt(r.PostFormValue("world_id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/worlds", "Mundo invalido.")
		return
	}

	// El recurso puede faltar: "que no se aplique ninguno" no nombra a nadie.
	resourceID, _ := strconv.ParseInt(r.PostFormValue("resource_id"), 10, 64)

	actor, ip := userFrom(r), clientIP(r)

	var (
		errAccion error
		mensaje   string
	)
	switch r.PostFormValue("accion") {
	case "quitar":
		errAccion = s.recursos.QuitarDeMundo(r.Context(), actor, worldID, resourceID, ip)
		mensaje = "Quitado de este mundo. Sigue en la biblioteca."
	case "principal":
		errAccion = s.recursos.HacerPrincipal(r.Context(), actor, worldID, resourceID, ip)
		mensaje = "Listo. Se aplica en el proximo arranque del servidor."
	case "obligatorio":
		errAccion = s.recursos.SetRequerido(r.Context(), actor, worldID,
			r.PostFormValue("required") != "", ip)
		mensaje = "Guardado. Se aplica en el proximo arranque del servidor."
	default:
		s.redirectError(w, r, "/worlds", "Accion desconocida.")
		return
	}

	if errAccion != nil {
		s.redirectError(w, r, "/worlds", s.resourceError(errAccion))
		return
	}
	s.responderRecursos(w, r, worldID, mensaje)
}

// responderRecursos devuelve el dialogo repintado, o recarga si no hay JS.
//
// Se repinta el CUERPO entero y no una fila: cambiar cual se aplica solo toca
// tambien al que dejaba de aplicarse, quitar uno cambia la lista de los que
// quedan por elegir, y anadir uno puede agotar el tope. Nada de eso cabe en una
// fila.
//
// Sin JavaScript esto no se ejecuta: el formulario se envia como siempre y el
// navegador recarga. La cabecera la manda el propio script, para que
// sustituirlo por HTMX de verdad no obligue a tocar nada del servidor.
func (s *Server) responderRecursos(w http.ResponseWriter, r *http.Request, worldID int64, mensaje string) {
	if !esParcial(r) {
		s.redirectInfo(w, r, "/worlds", mensaje)
		return
	}

	datos, err := s.recursosDeUnMundo(r, userFrom(r), worldID)
	if err != nil {
		s.log.Error("no se pudo repintar los recursos del mundo", "error", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	if err := s.renderer.fragment(w, "worlds.html", "recursos-cuerpo", datos); err != nil {
		s.log.Error("no se pudo pintar el dialogo de recursos", "error", err)
	}
}

func (s *Server) resourceError(err error) string {
	switch {
	case errors.Is(err, domain.ErrResourceURLInvalida):
		return "El enlace tiene que empezar por https://"
	case errors.Is(err, domain.ErrResourceDuplicado):
		return "Ya hay un recurso con ese mismo enlace."
	case errors.Is(err, domain.ErrResourceYaEnMundo):
		return "Este mundo ya lleva ese recurso."
	case errors.Is(err, domain.ErrDemasiadosRecursos):
		return fmt.Sprintf("Un mundo admite como mucho %d recursos. "+
			"Quita alguno antes de anadir otro.", domain.MaxRecursosPorMundo)
	case errors.Is(err, domain.ErrResourceNoAutomatico):
		return "Ese enlace lleva a una pagina, no al archivo. Puede ir en la lista, " +
			"pero no puede aplicarse solo: para eso hace falta uno que acabe en .zip"
	case errors.Is(err, domain.ErrResourcePrincipalNoAsignado):
		return "Marcaste como principal un recurso que no esta en la lista."
	case errors.Is(err, domain.ErrResourceNotFound):
		return "Ese recurso ya no existe."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con los recursos", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
