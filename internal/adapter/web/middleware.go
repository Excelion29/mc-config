package web

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Excelion29/mc-config/internal/domain"
)

type contextKey string

const userKey contextKey = "user"

// withSession resuelve el usuario de la cookie y lo deja en el contexto.
// No corta la peticion: decidir es tarea de requireSession y requireRole.
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		u, err := s.auth.UserFromSession(r.Context(), cookie.Value)
		if err != nil {
			// Sesion invalida: se borra la cookie para no reintentar en cada
			// peticion con un token que ya no sirve.
			s.clearCookie(w)
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireSession exige haber iniciado sesion.
func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requirePermission exige un permiso concreto (RBAC).
//
// Se pide el permiso, no el rol: asi crear un rol nuevo desde el panel no
// obliga a tocar ninguna ruta. La ruta declara QUE hace falta poder hacer; que
// rol lo concede es un dato editable.
func (s *Server) requirePermission(perm domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r)
			if u == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if !u.Can(perm) {
				s.renderer.render(w, http.StatusForbidden, "error.html", PageData{
					Title: "Sin permiso",
					User:  u,
					Error: "No tienes permiso para ver esta seccion.",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// chiParam envuelve el acceso a los parametros de ruta, para que el resto del
// adaptador no dependa directamente del router.
func chiParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

func userFrom(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userKey).(*domain.User)
	return u
}

// clientIP obtiene la IP del cliente.
//
// El panel esta detras de Nginx Proxy Manager, asi que RemoteAddr seria siempre
// la del proxy. Se usa X-Forwarded-For, que NPM rellena.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// El primer valor es el cliente original; el resto son proxies.
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
