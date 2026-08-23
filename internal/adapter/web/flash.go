package web

import (
	"net/http"
	"net/url"
	"strings"
)

// Mensajes de un solo uso entre peticiones.
//
// Existen para poder aplicar POST/Redirect/GET: tras una accion se redirige en
// vez de pintar la pagina directamente. Sin eso, la URL se queda en la ruta del
// POST y recargar reenvia el formulario -o, como paso aqui, el auto-refresco
// lanza un GET contra una ruta que solo acepta POST y responde 405.
//
// Se usa una cookie y no la sesion en base porque el mensaje vive dos segundos
// y no merece una escritura en disco.
const flashCookie = "mcvps_flash"

type flashKind string

const (
	flashInfo  flashKind = "i"
	flashError flashKind = "e"
)

func (s *Server) setFlash(w http.ResponseWriter, kind flashKind, text string) {
	if text == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookie,
		Value:    string(kind) + ":" + url.QueryEscape(text),
		Path:     "/",
		MaxAge:   30,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// takeFlash lee el mensaje y lo borra: debe verse una sola vez.
func (s *Server) takeFlash(w http.ResponseWriter, r *http.Request) (info, errMsg string) {
	c, err := r.Cookie(flashCookie)
	if err != nil || c.Value == "" {
		return "", ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     flashCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	kind, raw, ok := strings.Cut(c.Value, ":")
	if !ok {
		return "", ""
	}
	text, err := url.QueryUnescape(raw)
	if err != nil {
		return "", ""
	}

	if flashKind(kind) == flashError {
		return "", text
	}
	return text, ""
}

// redirectInfo deja un mensaje y redirige. 303 obliga al navegador a usar GET,
// que es justo lo que evita el reenvio del formulario al recargar.
func (s *Server) redirectInfo(w http.ResponseWriter, r *http.Request, to, msg string) {
	s.setFlash(w, flashInfo, msg)
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (s *Server) redirectError(w http.ResponseWriter, r *http.Request, to, msg string) {
	s.setFlash(w, flashError, msg)
	http.Redirect(w, r, to, http.StatusSeeOther)
}
