package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/domain"
)

const rutaPacks = "/packs"

func (s *Server) showPacks(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	info, errMsg := s.takeFlash(w, r)

	lista, err := s.packs.List(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Paquetes", "No se pudo leer la biblioteca.", err)
		return
	}

	s.renderer.render(w, http.StatusOK, "packs.html", packsPageData{
		PageData: s.pagina(r, "Paquetes", errMsg, info),
		Packs:    vistasDePack(lista),
	})
}

func (s *Server) createPack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaPacks, "No se pudo leer el formulario.")
		return
	}

	_, err := s.packs.Create(r.Context(), userFrom(r),
		r.PostFormValue("name"), r.PostFormValue("url"), r.PostFormValue("note"), clientIP(r))
	if err != nil {
		s.redirectError(w, r, rutaPacks, s.packError(err))
		return
	}
	s.redirectInfo(w, r, rutaPacks, "Paquete anadido.")
}

func (s *Server) updatePack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaPacks, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaPacks, "Paquete invalido.")
		return
	}

	if err := s.packs.Update(r.Context(), userFrom(r), id,
		r.PostFormValue("name"), r.PostFormValue("url"), r.PostFormValue("note"), clientIP(r)); err != nil {
		s.redirectError(w, r, rutaPacks, s.packError(err))
		return
	}
	s.redirectInfo(w, r, rutaPacks, "Paquete actualizado.")
}

func (s *Server) deletePack(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaPacks, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaPacks, "Paquete invalido.")
		return
	}

	if err := s.packs.Delete(r.Context(), userFrom(r), id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaPacks, s.packError(err))
		return
	}
	s.redirectInfo(w, r, rutaPacks, "Paquete borrado. Los mundos que lo llevaban se quedan sin el.")
}

// assignPacks fija que paquetes lleva un mundo y cual se aplica solo.
func (s *Server) assignPacks(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	worldID, err := strconv.ParseInt(r.PostFormValue("world_id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/worlds", "Mundo invalido.")
		return
	}

	// El activo puede no venir: un mundo puede listar paquetes para instalar a
	// mano sin que ninguno se aplique solo.
	activo, _ := strconv.ParseInt(r.PostFormValue("activo"), 10, 64)

	var ids []int64
	for _, v := range r.PostForm["pack_id"] {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	if err := s.packs.Asignar(r.Context(), userFrom(r), worldID, ids, activo,
		r.PostFormValue("required") != "", clientIP(r)); err != nil {
		s.redirectError(w, r, "/worlds", s.packError(err))
		return
	}
	s.redirectInfo(w, r, "/worlds",
		"Paquetes guardados. Se aplican en el proximo arranque del servidor.")
}

func (s *Server) packError(err error) string {
	switch {
	case errors.Is(err, domain.ErrPackURLInvalida):
		return "El enlace tiene que empezar por https://"
	case errors.Is(err, domain.ErrPackDuplicado):
		return "Ya hay un paquete con ese mismo enlace."
	case errors.Is(err, domain.ErrPackNoAutomatico):
		return "Ese enlace lleva a una pagina, no al archivo. Puede ir en la lista, " +
			"pero no puede aplicarse solo: para eso hace falta un enlace que acabe en .zip"
	case errors.Is(err, domain.ErrPackActivoNoAsignado):
		return "Marcaste como automatico un paquete que no esta en la lista."
	case errors.Is(err, domain.ErrPackNotFound):
		return "Ese paquete ya no existe."
	case errors.Is(err, domain.ErrEmptyName):
		return "Ponle un nombre al paquete."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con los paquetes", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
