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

// notFound responde como si la ruta no existiera. Es la respuesta tanto para
// una URL inventada como para una seccion que el usuario no puede ver: desde
// fuera no se distinguen, que es justo lo que se busca.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.renderer.render(w, http.StatusNotFound, "error.html", PageData{
		Title: "No encontrado",
		User:  userFrom(r),
		Error: "Esa pagina no existe.",
	})
}

// requirePermission exige un permiso concreto (RBAC).
//
// Se pide el permiso, no el rol: asi crear un rol nuevo desde el panel no
// obliga a tocar ninguna ruta. La ruta declara QUE hace falta poder hacer; que
// rol lo concede es un dato editable.
// requireAnyPermission deja pasar a quien tenga AL MENOS uno de los permisos.
//
// Hace falta donde una pantalla junta dos cosas que se gestionan por separado.
// La de Acceso es el caso: quien administra jugadores entra a la lista, y quien
// opera el servidor entra al interruptor. Exigir los dos dejaria fuera a la
// mitad de cada uno; exigir solo uno cerraria la puerta al otro.
//
// Lo que cada uno VE dentro sigue decidiendolo la plantilla, y lo que puede
// HACER lo decide el caso de uso. Esto solo abre la puerta.
func (s *Server) requireAnyPermission(perms ...domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r)
			if u == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			for _, p := range perms {
				if u.Can(p) {
					next.ServeHTTP(w, r)
					return
				}
			}
			// 404 por el mismo motivo que en requirePermission.
			s.notFound(w, r)
		})
	}
}

func (s *Server) requirePermission(perm domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := userFrom(r)
			if u == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			if !u.Can(perm) {
				// 404 y no 403, a proposito: "no tienes permiso" CONFIRMA que
				// el recurso existe. Quien no puede usar una seccion tampoco
				// deberia poder deducir que esta ahi. La respuesta es
				// indistinguible de la de una ruta inventada.
				s.notFound(w, r)
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
