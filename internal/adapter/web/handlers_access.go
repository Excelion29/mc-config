package web

import (
	"errors"
	"net/http"

	"github.com/Excelion29/mc-config/internal/domain"
)

const rutaAcceso = "/access"

// showAccess junta las dos mitades de una misma pregunta.
//
// El modo dice SI pueden entrar cuentas no premium; la lista dice QUIENES
// entran. Tenerlas en pantallas distintas obligaba a ir y volver para algo que
// se decide de una vez.
//
// Cada mitad la ve quien tiene su permiso, y quien tenga uno solo ve media
// pantalla sin enterarse de que existe la otra.
func (s *Server) showAccess(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	info, errMsg := s.takeFlash(w, r)

	datos := accessPageData{
		PageData: s.pagina(r, "Acceso", errMsg, info),
		Puede:    actor.Can(domain.PermServerOperate),
	}

	if datos.Puede {
		estado, err := s.acceso.Estado(r.Context(), actor)
		if err != nil {
			s.renderFailure(w, actor, "Acceso", "No se pudo leer el estado del acceso.", err)
			return
		}
		datos.Estado = estado
	}

	jugadores, err := s.listaDeJugadores(r, actor)
	if err != nil {
		s.renderFailure(w, actor, "Acceso", "No se pudo leer la lista de jugadores.", err)
		return
	}
	datos.Jugadores = jugadores

	s.renderer.render(w, http.StatusOK, "access.html", datos)
}

// installPlugins descarga e instala lo que hace falta, SIN activar nada.
//
// Son dos botones y no uno a proposito: instalar no cambia el comportamiento
// del servidor, activar si. Juntarlos escondería el momento en que se abre la
// puerta.
func (s *Server) installPlugins(w http.ResponseWriter, r *http.Request) {
	if err := s.acceso.PrepararPlugins(r.Context(), userFrom(r), clientIP(r)); err != nil {
		s.redirectError(w, r, rutaAcceso, s.accessError(err))
		return
	}
	s.redirectInfo(w, r, rutaAcceso,
		"Complementos instalados. Se cargaran la proxima vez que arranques el servidor.")
}

func (s *Server) setAuthMode(w http.ResponseWriter, r *http.Request) {
	modo := domain.AuthMode(r.PostFormValue("mode"))

	if err := s.acceso.SetMode(r.Context(), userFrom(r), modo, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaAcceso, s.accessError(err))
		return
	}

	msg := "A partir del proximo arranque solo entraran cuentas compradas."
	if modo.SinConexion() {
		msg = "A partir del proximo arranque tambien entraran cuentas no premium, " +
			"con contrasena. Reinicia el servidor para aplicarlo."
	}
	s.redirectInfo(w, r, rutaAcceso, msg)
}

func (s *Server) accessError(err error) string {
	switch {
	case errors.Is(err, domain.ErrPluginsMissing):
		return "Faltan complementos. Instalalos antes de abrir el acceso."
	case errors.Is(err, domain.ErrNoJavaInstance):
		return "No hay ningun servidor de Java. Crea uno primero."
	case errors.Is(err, domain.ErrPluginsUnavailable):
		return "El panel no puede instalar complementos: revisa MCVPS_PLUGINS_PATH."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado en el acceso", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
