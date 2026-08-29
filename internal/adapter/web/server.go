// Package web es el adaptador HTTP: traduce peticiones a casos de uso de app.
//
// No contiene reglas de negocio. Si aqui aparece una decision sobre quien puede
// hacer que, esta en el sitio equivocado: va en domain.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

//go:embed templates/*.html static/css/*.css static/js/*.js static/favicon.svg
var assets embed.FS

const cookieName = "mcvps_session"

type Server struct {
	auth      *app.Auth
	audit     *app.Audit
	worlds    *app.Worlds
	packs     *app.Packs
	acceso    *app.Access
	instances *app.Instances
	players   *app.Players
	renderer  *renderer
	log       *slog.Logger

	// secureCookies marca la cookie como Secure. Debe ir a true en produccion,
	// donde NPM sirve el panel por HTTPS (D-10).
	secureCookies bool
	sessionTTL    time.Duration
}

func NewServer(
	auth *app.Auth,
	audit *app.Audit,
	worlds *app.Worlds,
	packs *app.Packs,
	acceso *app.Access,
	instances *app.Instances,
	players *app.Players,
	log *slog.Logger,
	secureCookies bool,
	sessionTTL time.Duration,
) (*Server, error) {
	r, err := newRenderer(assets)
	if err != nil {
		return nil, err
	}
	return &Server{
		auth: auth, audit: audit, worlds: worlds, packs: packs, acceso: acceso, instances: instances, players: players,
		renderer: r, log: log,
		secureCookies: secureCookies, sessionTTL: sessionTTL,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.securityHeaders)
	r.Use(s.withSession)

	static, err := fs.Sub(assets, "static")
	if err != nil {
		panic(fmt.Sprintf("no se pudo montar /static: %v", err))
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	// Sonda para el healthcheck de Docker. Sin sesion a proposito.
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/login", s.showLogin)
	r.Post("/login", s.handleLogin)
	r.Post("/logout", s.handleLogout)

	r.Group(func(private chi.Router) {
		private.Use(s.requireSession)
		private.Get("/", s.home)

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermServerView))
			g.Get("/worlds", s.showWorlds)
			g.Get("/worlds/{id}/icon", s.worldIcon)
			g.Get("/packs", s.showPacks)
			g.Get("/access", s.showAccess)
			g.Get("/instances", s.showInstances)
			g.Get("/instances/{id}/logs", s.instanceLogs)
			// Flujo en vivo para la consola. Se queda abierto mientras el
			// dialogo este abierto, por eso NO puede compartir el tiempo
			// limite de escritura del resto de rutas.
			g.Get("/instances/{id}/logs/stream", s.streamLogs)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermServerOperate))
			g.Post("/access/plugins", s.installPlugins)
			g.Post("/access/mode", s.setAuthMode)
			g.Post("/instances/start", s.startInstance)
			g.Post("/instances/stop", s.stopInstance)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermInstanceCreate))
			g.Post("/instances", s.createInstance)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermInstanceDelete))
			g.Post("/instances/delete", s.deleteInstance)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermWorldImport))
			g.Post("/worlds", s.importWorld)
			g.Post("/worlds/create", s.createWorld)
			g.Post("/worlds/update", s.updateWorld)
			g.Post("/worlds/packs", s.assignPacks)
			g.Post("/packs", s.createPack)
			g.Post("/packs/update", s.updatePack)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermWorldDelete))
			g.Post("/worlds/delete", s.deleteWorld)
			g.Post("/packs/delete", s.deletePack)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermPlayerManage))
			g.Get("/players", s.showPlayers)
			g.Post("/players", s.addPlayer)
			g.Post("/players/active", s.setPlayerActive)
			g.Post("/players/op", s.setPlayerOp)
			g.Post("/players/delete", s.deletePlayer)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermAuditView))
			g.Get("/audit", s.showAudit)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermUserManage))
			g.Get("/users", s.showUsers)
			g.Post("/users", s.createUser)
			g.Post("/users/active", s.setUserActive)
			g.Post("/users/role", s.setUserRole)
		})

		private.Group(func(g chi.Router) {
			g.Use(s.requirePermission(domain.PermRoleManage))
			g.Get("/roles", s.showRoles)
			g.Post("/roles", s.createRole)
			g.Post("/roles/permissions", s.setRolePermissions)
			g.Post("/roles/delete", s.deleteRole)
		})
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.renderer.render(w, http.StatusNotFound, "error.html", PageData{
			Title: "No encontrado",
			User:  userFrom(r),
			Error: "Esa pagina no existe.",
		})
	})

	return r
}

// securityHeaders fija cabeceras defensivas.
//
// La CSP no permite scripts externos ni en linea: cuando HTMX entre en F3, ira
// como archivo propio bajo /static, no desde una CDN.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			// img-src admite https: para las portadas por enlace. Es la
			// relajacion mas barata de la politica: una imagen no ejecuta
			// nada, y sin scripts de terceros no sirve como via de fuga.
			// script-src y style-src siguen cerrados a 'self', que es donde
			// esta el peligro de verdad.
			//
			// Solo https, no http: la pagina va por TLS y el navegador
			// bloquearia la imagen por contenido mixto de todas formas.
			"default-src 'self'; img-src 'self' data: https:; style-src 'self'; script-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true, // inalcanzable desde JavaScript
		Secure:   s.secureCookies,
		// Lax impide que la cookie viaje en peticiones POST venidas de otro
		// sitio, que es la proteccion CSRF que necesitan las acciones del
		// panel. En F3, con acciones destructivas, se anadira ademas un token
		// CSRF por formulario.
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}
