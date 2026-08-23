package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/domain"
)

const rutaJugadores = "/players"

func (s *Server) showPlayers(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	info, errMsg := s.takeFlash(w, r)

	list, err := s.players.List(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Jugadores", "No se pudo leer la lista de jugadores.", err)
		return
	}

	s.renderer.render(w, http.StatusOK, "players.html", playersPageData{
		PageData: PageData{Title: "Jugadores", User: actor, Error: errMsg, Info: info},
		Players:  list,
	})
}

func (s *Server) addPlayer(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaJugadores, "No se pudo leer el formulario.")
		return
	}

	p, err := s.players.Add(r.Context(), actor,
		r.PostFormValue("gamertag"), r.PostFormValue("note"),
		r.PostFormValue("is_op") == "1", clientIP(r))
	if err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}

	s.redirectInfo(w, r, rutaJugadores,
		p.Gamertag+" ya puede entrar a todos los servidores.")
}

func (s *Server) setPlayerActive(w http.ResponseWriter, r *http.Request) {
	id, active, ok := s.playerForm(w, r, "active")
	if !ok {
		return
	}

	if err := s.players.SetActive(r.Context(), userFrom(r), id, active, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}

	msg := "Jugador bloqueado: ya no puede entrar."
	if active {
		msg = "Jugador permitido."
	}
	s.redirectInfo(w, r, rutaJugadores, msg)
}

func (s *Server) setPlayerOp(w http.ResponseWriter, r *http.Request) {
	id, isOp, ok := s.playerForm(w, r, "is_op")
	if !ok {
		return
	}

	if err := s.players.SetOp(r.Context(), userFrom(r), id, isOp, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}
	s.redirectInfo(w, r, rutaJugadores, "Permisos de operador actualizados.")
}

func (s *Server) deletePlayer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaJugadores, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaJugadores, "Jugador invalido.")
		return
	}

	if err := s.players.Delete(r.Context(), userFrom(r), id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}
	s.redirectInfo(w, r, rutaJugadores, "Jugador borrado de la lista.")
}

// playerForm lee el id y un valor booleano del formulario.
func (s *Server) playerForm(w http.ResponseWriter, r *http.Request, campo string) (int64, bool, bool) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaJugadores, "No se pudo leer el formulario.")
		return 0, false, false
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaJugadores, "Jugador invalido.")
		return 0, false, false
	}
	return id, r.PostFormValue(campo) == "1", true
}

func (s *Server) playerError(err error) string {
	switch {
	case errors.Is(err, domain.ErrDuplicatePlayer):
		return "Ese gamertag ya esta en la lista."
	case errors.Is(err, domain.ErrEmptyGamertag):
		return "El gamertag es obligatorio."
	case errors.Is(err, domain.ErrPlayerNotFound):
		return "Ese jugador ya no esta en la lista."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con un jugador", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
