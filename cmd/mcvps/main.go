// Comando mcvps: panel web de gestion de servidores Minecraft.
//
// Este archivo es la raiz de composicion: el unico sitio donde se decide que
// implementacion concreta cubre cada puerto. El resto del codigo solo conoce
// interfaces, que es lo que permite cambiar de motor de base de datos o anadir
// el adaptador de Java (D-01) sin tocar los casos de uso.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Excelion29/mc-config/internal/adapter/bedrock"
	"github.com/Excelion29/mc-config/internal/adapter/dockerx"
	"github.com/Excelion29/mc-config/internal/adapter/mcworld"
	"github.com/Excelion29/mc-config/internal/adapter/security"
	"github.com/Excelion29/mc-config/internal/adapter/sqlite"
	"github.com/Excelion29/mc-config/internal/adapter/storage"
	"github.com/Excelion29/mc-config/internal/adapter/web"
	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/config"
)

func main() {
	// La imagen de produccion es distroless: no hay shell ni curl con los que
	// hacer el healthcheck de Docker. El propio binario se sondea a si mismo.
	probe := flag.Bool("healthcheck", false, "comprueba /health y termina")
	seedFlag := flag.Bool("seed", false, "crea el rol y la cuenta de superusuario y termina")
	nueva := flag.String("new-migration", "", "crea un archivo de migracion vacio y termina")
	verMigraciones := flag.Bool("migrations", false, "muestra el estado de las migraciones y termina")
	flag.Parse()

	if *nueva != "" {
		os.Exit(newMigration(*nueva))
	}

	if *probe {
		os.Exit(checkHealth())
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *verMigraciones {
		os.Exit(listMigrations())
	}

	if *seedFlag {
		os.Exit(runSeed(log))
	}

	if err := run(log); err != nil {
		log.Error("el panel se detuvo por un error", "error", err)
		os.Exit(1)
	}
}

// deps agrupa todo lo que se construye en la raiz de composicion, para que el
// arranque normal y el sembrado compartan exactamente el mismo cableado.
type deps struct {
	cfg        config.Config
	close      func() error
	migrations func() (map[int]bool, error)
	auth       *app.Auth
	audit      *app.Audit
	maps       *app.Worlds
	instances  *app.Instances
	players    *app.Players
	// watcher aprende el XUID de cada jugador la primera vez que entra. Se
	// arma aqui, con el resto, para que la raiz de composicion siga siendo el
	// unico sitio donde se eligen implementaciones concretas.
	watcher *app.ConnectionWatcher
}

// build cablea todo. Acepta log nil para los comandos que no deben ensuciar la
// salida con lineas de arranque, como -migrations.
func build(log *slog.Logger) (*deps, error) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// --- Adaptadores -------------------------------------------------------
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	log.Info("base de datos lista", "ruta", cfg.DBPath)

	store, err := storage.New(cfg.WorldsPath)
	if err != nil {
		db.Close()
		return nil, err
	}
	log.Info("almacen de mundos listo", "ruta", cfg.WorldsPath)

	hasher := security.NewBcrypt()
	tokens := security.NewTokenGenerator()
	clock := app.Clock(func() time.Time { return time.Now().UTC() })

	// --- Casos de uso ------------------------------------------------------
	audit := app.NewAudit(db.Audit(), clock, log)
	auth := app.NewAuth(
		db.Users(), db.Roles(), db.Sessions(), hasher, tokens,
		audit, clock, cfg.SessionTTL, log,
	)

	inspector := mcworld.New()
	maps := app.NewWorlds(db.Worlds(), store, inspector, audit, clock, cfg.MaxUpload, log)

	// El runtime de contenedores puede no estar disponible al desarrollar. No
	// se aborta el arranque por eso: el panel sigue sirviendo para gestionar
	// mapas, usuarios y roles, y la seccion de servidores avisara al fallar.
	runtime, err := dockerx.NewRuntime(cfg.DockerHost)
	if err != nil {
		log.Warn("sin acceso a Docker; la gestion de servidores no funcionara", "error", err)
	} else if err := runtime.Ping(context.Background()); err != nil {
		log.Warn("Docker no responde; la gestion de servidores no funcionara", "error", err)
	} else {
		log.Info("Docker disponible")
	}

	instances := app.NewInstances(
		db.Instances(), db.Worlds(), store, runtime,
		[]app.ServerFlavor{bedrock.New(inspector)},
		audit, clock, cfg.InstancesPath, cfg.GameHost, log,
	)

	players := app.NewPlayers(db.Players(), instances, audit, clock, log)
	watcher := app.NewConnectionWatcher(db.Players(), instances, bedrock.New(inspector), log)

	// Ciclo cerrado a proposito: Players necesita Instances para propagar la
	// lista, e Instances necesita la lista al arrancar. Se inyecta despues de
	// construir ambos, en vez de acoplarlos entre si.
	instances.SetAllowlistSource(players.ActiveGamertags)

	return &deps{
		cfg:        cfg,
		close:      db.Close,
		migrations: func() (map[int]bool, error) { return db.AppliedMigrations(context.Background()) },
		auth:       auth,
		audit:      audit,
		maps:       maps,
		instances:  instances,
		players:    players,
		watcher:    watcher,
	}, nil
}

func run(log *slog.Logger) error {
	d, err := build(log)
	if err != nil {
		return err
	}
	defer d.close()

	cfg, auth, audit := d.cfg, d.auth, d.audit

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Lo unico que el arranque impone: el rol raiz y su unica cuenta. Los demas
	// roles y usuarios se crean desde el panel.
	if err := auth.EnsureSuperuser(ctx, cfg.SuperuserEmail, cfg.SuperuserPassword); err != nil {
		return err
	}
	if n, err := auth.PurgeSessions(ctx); err != nil {
		log.Warn("no se pudieron purgar sesiones caducadas", "error", err)
	} else if n > 0 {
		log.Info("sesiones caducadas eliminadas", "cantidad", n)
	}

	// --- Vigilante de conexiones -------------------------------------------
	//
	// Aprende el XUID de cada jugador la primera vez que entra, que es lo
	// unico que permite darle operador despues (ver 0008_xuid_de_jugadores).
	// Se cancela solo cuando se apaga el panel.
	go d.watcher.Run(ctx)

	// --- Adaptador HTTP ----------------------------------------------------
	handler, err := web.NewServer(auth, audit, d.maps, d.instances, d.players, log, cfg.SecureCookies, cfg.SessionTTL)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	failures := make(chan error, 1)
	go func() {
		log.Info("panel escuchando", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			failures <- err
		}
	}()

	select {
	case err := <-failures:
		return err
	case <-ctx.Done():
		log.Info("apagando el panel")
	}

	// Apagado ordenado: se deja terminar a las peticiones en curso. Es el mismo
	// criterio que exigimos al servidor de Minecraft en F3 (H-F0-6): nunca
	// cortar a lo bruto.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("panel detenido correctamente")
	return nil
}

// checkHealth consulta /health en la propia instancia. Devuelve el codigo de
// salida que espera el HEALTHCHECK de Docker: 0 sana, 1 enferma.
func checkHealth() int {
	addr := os.Getenv("MCVPS_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: estado", resp.StatusCode)
		return 1
	}
	return 0
}
