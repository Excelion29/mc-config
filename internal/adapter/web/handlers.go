package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

func (s *Server) showLogin(w http.ResponseWriter, r *http.Request) {
	if userFrom(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderer.render(w, http.StatusOK, "login.html", PageData{Title: "Entrar"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderer.render(w, http.StatusBadRequest, "login.html", PageData{
			Title: "Entrar",
			Error: "No se pudo leer el formulario.",
		})
		return
	}

	email := r.PostFormValue("email")
	password := r.PostFormValue("password")

	session, user, err := s.auth.Login(r.Context(), email, password, clientIP(r))
	if err != nil {
		// Se muestra el mismo mensaje para credenciales malas y usuario
		// inexistente. Distinguirlos permitiria descubrir que correos existen
		// probando uno a uno.
		msg := "Correo o contrasena incorrectos."
		if errors.Is(err, domain.ErrUserDisabled) {
			msg = "Esta cuenta esta desactivada."
		} else if !errors.Is(err, domain.ErrInvalidCredentials) {
			s.log.Error("fallo inesperado en login", "error", err)
			msg = "Ha ocurrido un error. Intentalo de nuevo."
		}

		s.renderer.render(w, http.StatusUnauthorized, "login.html", PageData{
			Title: "Entrar",
			Error: msg,
			Email: app.NormalizeEmail(email), // no se devuelve la contrasena
		})
		return
	}

	s.setCookie(w, session.Token, session.ExpiresAt)
	s.log.Info("sesion iniciada", "email", user.Email, "rol", user.Role)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		if err := s.auth.Logout(r.Context(), cookie.Value, userFrom(r), clientIP(r)); err != nil {
			s.log.Error("fallo cerrando sesion", "error", err)
		}
	}
	s.clearCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	s.renderer.render(w, http.StatusOK, "home.html", PageData{
		Title: "Panel",
		User:  userFrom(r),
	})
}

func (s *Server) showAudit(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)

	// Los filtros viajan en la URL, no en una sesion: asi una busqueda se
	// puede compartir o guardar en marcadores, y el boton "atras" deshace el
	// filtro en vez de dejar la pantalla en un estado que nadie pidio.
	q := r.URL.Query()
	pagina, _ := strconv.Atoi(q.Get("p"))

	page, err := s.audit.Search(r.Context(), u, app.AuditFilter{
		Text:   q.Get("q"),
		Action: domain.Action(q.Get("accion")),
		Page:   pagina,
		Size:   25,
	})
	if err != nil {
		s.log.Error("no se pudo leer el registro", "error", err)
		s.renderer.render(w, http.StatusInternalServerError, "error.html", PageData{
			Title: "Registro",
			User:  u,
			Error: "No se pudo leer el registro de acciones.",
		})
		return
	}

	s.renderer.render(w, http.StatusOK, "audit.html", auditPageData{
		PageData: PageData{Title: "Registro", User: u},
		Page:     page,
		Actions:  app.AuditActions(),
		Pag: paginador{
			Info: page.PageInfo,
			// Los filtros van dentro de la base para que pasar de pagina no
			// los pierda. url.Values los escapa: un correo con "&" o un
			// detalle con espacios romperian la URL a mano.
			Base: "/audit?" + url.Values{
				"q":      {page.Filter.Text},
				"accion": {string(page.Filter.Action)},
			}.Encode() + "&",
		},
	})
}
