package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// Todo lo de jugadores vuelve a la pantalla de Acceso: son la misma cosa. Las
// rutas de escritura siguen bajo /players porque son acciones sobre jugadores,
// no sobre el modo.
const rutaJugadores = rutaAcceso

// redirectToAccess manda /players a la pantalla que ahora los contiene.
//
// Se conserva la ruta para no romper enlaces guardados ni las vueltas de
// formularios viejos: es una redireccion, no un 404.
func (s *Server) redirectToAccess(w http.ResponseWriter, r *http.Request) {
	destino := rutaAcceso
	if q := r.URL.RawQuery; q != "" {
		destino += "?" + q
	}
	http.Redirect(w, r, destino, http.StatusMovedPermanently)
}

// listaDeJugadores arma la mitad derecha de la pantalla de Acceso.
//
// Devuelve la parte vacia -y sin error- para quien no gestione jugadores: esa
// persona entra por el interruptor y no tiene por que ver la lista.
func (s *Server) listaDeJugadores(r *http.Request, actor *domain.User) (jugadoresEnAcceso, error) {
	if !actor.Can(domain.PermPlayerManage) {
		return jugadoresEnAcceso{}, nil
	}

	q := r.URL.Query()
	pagina, _ := strconv.Atoi(q.Get("p"))
	filtro := app.PlayerFilter{Text: q.Get("q"), Estado: q.Get("estado")}

	page, err := s.players.SearchPage(r.Context(), actor, filtro, app.Paging{Page: pagina})
	if err != nil {
		return jugadoresEnAcceso{}, err
	}

	return jugadoresEnAcceso{
		Puede:   true,
		Players: vistasDeJugador(page.Players, s.acceso.Mode(r.Context())),
		Filtro:  page.Filter,
		Estados: app.EstadosDeJugador(),
		Pag: paginador{
			Info: page.PageInfo,
			// Los filtros viajan dentro de la base para no perderlos al pasar
			// de pagina. url.Values escapa lo que escriba el usuario.
			Base: rutaAcceso + "?" + url.Values{
				"q":      {filtro.Text},
				"estado": {filtro.Estado},
			}.Encode() + "&",
		},
	}, nil
}

// updatePlayer corrige las identidades de alguien ya dado de alta.
//
// Existe porque sin esto, anadirle el nombre de Java a quien se dio de alta solo
// con su gamertag obligaba a borrarlo y volver a crearlo, perdiendo su nota y su
// historial por el camino.
func (s *Server) updatePlayer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaJugadores, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaJugadores, "Jugador invalido.")
		return
	}

	if err := s.players.Update(r.Context(), userFrom(r), id,
		r.PostFormValue("gamertag"), r.PostFormValue("java_name"),
		r.PostFormValue("note"), clientIP(r)); err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}
	s.redirectInfo(w, r, rutaJugadores, "Jugador actualizado.")
}

func (s *Server) addPlayer(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaJugadores, "No se pudo leer el formulario.")
		return
	}

	p, err := s.players.Add(r.Context(), actor,
		r.PostFormValue("gamertag"), r.PostFormValue("java_name"), r.PostFormValue("note"),
		r.PostFormValue("is_op") == "1", clientIP(r))
	if err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}

	s.redirectInfo(w, r, rutaJugadores,
		p.Gamertag+" ya puede entrar a todos los servidores.")
}

// esParcial distingue una peticion que quiere solo el fragmento.
//
// La cabecera es la que manda HTMX, para que sustituirlo por el HTMX de verdad
// no obligue a tocar nada del servidor.
func esParcial(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// responderFila devuelve la fila recién cambiada, ya con su estado nuevo.
//
// Se relee el jugador de la base en vez de construirlo con lo que se acaba de
// enviar: lo que vale es lo que quedo guardado, no lo que el navegador creia
// estar pidiendo.
func (s *Server) responderFila(w http.ResponseWriter, r *http.Request, id int64) {
	actor := userFrom(r)

	p, err := s.players.ByID(r.Context(), actor, id)
	if err != nil {
		s.log.Error("no se pudo releer el jugador", "id", id, "error", err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	s.ponerTotal(w, r, actor)
	fila := vistaDeJugador(p, s.acceso.Mode(r.Context()))
	if err := s.renderer.fragment(w, "access.html", "fila-jugador", fila); err != nil {
		s.log.Error("no se pudo pintar la fila", "error", err)
	}
}

// ponerTotal manda el total en una cabecera para que el contador del titulo se
// actualice. Contar filas en el navegador daria un numero falso en cuanto haya
// paginacion: en pantalla hay 25, en la lista 300.
func (s *Server) ponerTotal(w http.ResponseWriter, r *http.Request, actor *domain.User) {
	page, err := s.players.ListPage(r.Context(), actor, app.Paging{})
	if err != nil {
		return
	}
	w.Header().Set("X-Total", strconv.Itoa(page.Total))
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

	if esParcial(r) {
		s.responderFila(w, r, id)
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
	if esParcial(r) {
		s.responderFila(w, r, id)
		return
	}
	s.redirectInfo(w, r, rutaJugadores, "Permisos de operador actualizados.")
}

func (s *Server) deletePlayer(w http.ResponseWriter, r *http.Request) {
	// Sin ParseForm previo: PostFormValue ya interpreta el cuerpo, y ademas
	// sabe hacerlo con los DOS formatos. Llamar antes a ParseForm deja
	// PostForm inicializado y vacio para un cuerpo multipart, y entonces todos
	// los campos llegan en blanco sin que nada de error.
	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaJugadores, "Jugador invalido.")
		return
	}

	if err := s.players.Delete(r.Context(), userFrom(r), id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaJugadores, s.playerError(err))
		return
	}
	if esParcial(r) {
		// Cuerpo vacio: la fila se quita, no se sustituye. El total si viaja,
		// porque el contador del titulo tiene que bajar.
		s.ponerTotal(w, r, userFrom(r))
		w.WriteHeader(http.StatusOK)
		return
	}
	s.redirectInfo(w, r, rutaJugadores, "Jugador borrado de la lista.")
}

// playerForm lee el id y un valor booleano del formulario.
func (s *Server) playerForm(w http.ResponseWriter, r *http.Request, campo string) (int64, bool, bool) {
	// Ver deletePlayer: nada de ParseForm antes de PostFormValue.
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
		return "Indica al menos un nombre: el de Bedrock o el de Java."
	case errors.Is(err, domain.ErrJavaNameNotFound):
		return "Mojang no conoce esa cuenta de Java. Revisa como esta escrita."
	case errors.Is(err, domain.ErrPlayerNotFound):
		return "Ese jugador ya no esta en la lista."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con un jugador", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
