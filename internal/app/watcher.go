package app

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// LogScanner traduce una linea del log a una conexion. Lo implementa el
// adaptador de cada edicion, porque el formato del log es cosa suya.
type LogScanner interface {
	// ScanLine devuelve el gamertag y el XUID si la linea es una conexion.
	// El tercer valor distingue "no venia al caso" de "hablaba de un xuid
	// pero no la entendi", que es el caso que hay que mirar.
	ScanLine(linea string) (gamertag, xuid string, entendida, sospechosa bool)
}

// ConnectionWatcher aprende el XUID de los jugadores mirando el log del
// servidor encendido.
//
// Existe porque permissions.json identifica a los operadores por XUID, y el
// panel no lo conoce hasta que la persona entra por primera vez. Es la segunda
// fase del alta:
//
//	1. Se le anade a la allow-list por gamertag  -> ya puede entrar
//	2. Entra                                     -> aqui aprendemos su XUID
//	3. Solo entonces se le puede dar operador
//
// Que nadie fuera de la allow-list llegue a conectarse es lo que hace esto
// seguro: solo se capturan XUIDs de gente que ya habias aprobado.
type ConnectionWatcher struct {
	players   PlayerRepo
	instances *Instances
	scanner   LogScanner
	cada      time.Duration
	// lineas es cuantas del final se releen en cada vuelta. Ha de cubrir
	// holgadamente lo que un servidor escribe entre dos vueltas; si se queda
	// corto se pierden conexiones.
	lineas int
	log    *slog.Logger
}

func NewConnectionWatcher(players PlayerRepo, instances *Instances, scanner LogScanner, log *slog.Logger) *ConnectionWatcher {
	return &ConnectionWatcher{
		players:   players,
		instances: instances,
		scanner:   scanner,
		cada:      20 * time.Second,
		lineas:    300,
		log:       log,
	}
}

// Run sondea hasta que se cancele el contexto.
//
// Se sondea en vez de seguir el log en streaming porque una conexion abierta
// permanentemente contra Docker es una pieza mas que puede romperse -y que hay
// que reconectar, y vigilar-. Aqui no hace falta: solo interesan los jugadores
// que TODAVIA no tienen XUID, asi que releer las mismas lineas una y otra vez
// es inofensivo. La operacion es idempotente por construccion, y eso permite
// que la forma mas simple sea tambien la correcta.
func (w *ConnectionWatcher) Run(ctx context.Context) {
	t := time.NewTicker(w.cada)
	defer t.Stop()

	w.log.Info("vigilando conexiones para aprender XUIDs", "cada", w.cada)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.vuelta(ctx)
		}
	}
}

func (w *ConnectionWatcher) vuelta(ctx context.Context) {
	inst, err := w.instances.Running(ctx)
	if err != nil || inst == nil {
		return // nadie encendido: no hay log que mirar
	}

	logs, err := w.instances.RawLogs(ctx, inst, w.lineas)
	if err != nil {
		w.log.Debug("no se pudo leer el log", "instancia", inst.Name, "error", err)
		return
	}

	nuevos := 0
	for _, linea := range strings.Split(logs, "\n") {
		gamertag, xuid, entendida, sospechosa := w.scanner.ScanLine(linea)

		if sospechosa {
			// Mojang cambia el formato del log entre versiones sin avisar. Si
			// deja de encajar, esto es lo unico que lo delata: sin el aviso,
			// el sintoma seria "la estrella de operador no funciona" y nadie
			// sabria por que.
			w.log.Warn("linea de conexion no reconocida; puede que Mojang haya cambiado el formato",
				"linea", strings.TrimSpace(linea))
			continue
		}
		if !entendida {
			continue
		}

		cambio, err := w.players.MarkSeen(ctx, gamertag, xuid, time.Now())
		if err != nil {
			w.log.Error("no se pudo guardar el xuid", "gamertag", gamertag, "error", err)
			continue
		}
		if cambio {
			nuevos++
			w.log.Info("primera conexion de un jugador; ya se le puede dar operador",
				"gamertag", gamertag, "xuid", xuid)
		}
	}

	// Solo se reescribe si alguien se estreno. Sin esto, cada veinte segundos
	// se regeneraria un archivo identico y se mandaria un "permission reload"
	// al servidor sin motivo.
	if nuevos > 0 {
		w.propagar(ctx)
	}
}

// propagar reescribe permissions.json de todas las instancias.
func (w *ConnectionWatcher) propagar(ctx context.Context) {
	ops, err := w.players.Ops(ctx)
	if err != nil {
		w.log.Error("no se pudieron leer los operadores", "error", err)
		return
	}
	w.instances.ApplyOps(ctx, ops)
}

// OpsFrom traduce jugadores del dominio a lo que pide el archivo.
func OpsFrom(jugadores []domain.Player) []OpEntry {
	out := make([]OpEntry, 0, len(jugadores))
	for i := range jugadores {
		p := &jugadores[i]
		if !p.HaEntrado() {
			continue
		}
		out = append(out, OpEntry{XUID: p.XUID, Gamertag: p.Gamertag})
	}
	return out
}
