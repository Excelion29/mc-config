package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Cada cuanto se vuelve a mirar el log del contenedor.
//
// Dos segundos: un servidor de Minecraft no escribe tan rapido como para que
// se note mas fino, y bajar de aqui multiplica las llamadas a Docker sin que
// nadie lo agradezca.
const cadenciaLogs = 2 * time.Second

// lineasIniciales es cuanto historial se manda al abrir la consola.
const lineasIniciales = 200

// streamLogs emite el log en vivo con Server-Sent Events.
//
// SSE y no WebSocket a proposito: esto es un flujo de un solo sentido, del
// servidor al navegador, y nadie escribe de vuelta. Un WebSocket traeria un
// protocolo entero -actualizacion de conexion, marcos, ping/pong, una
// biblioteca- para no usar la mitad. Ademas SSE viaja sobre HTTP normal, asi
// que atraviesa Nginx Proxy Manager sin configurarle nada, y el navegador
// reconecta solo cuando se corta: eso ultimo, con WebSocket, habria que
// escribirlo a mano.
//
// La consola es de SOLO LECTURA. Mandar ordenes al servidor desde aqui seria
// otra cosa muy distinta -y mucho mas delicada- que merece su propia decision.
func (s *Server) streamLogs(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	id, err := strconv.ParseInt(chiParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "instancia invalida", http.StatusBadRequest)
		return
	}

	// El flusher es imprescindible: sin el, la respuesta se queda en el bufer
	// y el navegador no ve nada hasta que termina, que en un flujo continuo es
	// nunca.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "el servidor no puede emitir en vivo", http.StatusInternalServerError)
		return
	}

	// El servidor tiene WriteTimeout de 30 s, que es lo correcto para una
	// pagina normal y letal para un flujo: cortaria la consola cada medio
	// minuto. Se levanta SOLO en esta ruta en vez de quitarlo globalmente,
	// para que el resto siga protegido de clientes que leen despacio.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		s.log.Warn("no se pudo levantar el limite de escritura; la consola se cortara sola", "error", err)
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Nginx bufferiza por defecto y eso rompe el directo sin dar ningun error:
	// simplemente no llega nada hasta que se acumula bastante.
	h.Set("X-Accel-Buffering", "no")

	// Primer envio: el historial reciente, para que la consola no se abra en
	// blanco esperando a que el servidor diga algo.
	previas, err := s.logLines(r.Context(), actor, id, lineasIniciales)
	if err != nil {
		emitirEvento(w, "error", "No se pudo leer el log.")
		flusher.Flush()
		return
	}
	for _, l := range previas {
		emitirEvento(w, "linea", l)
	}
	flusher.Flush()

	t := time.NewTicker(cadenciaLogs)
	defer t.Stop()

	for {
		select {
		case <-r.Context().Done():
			// El navegador cerro la pestana o el dialogo. No es un error.
			return
		case <-t.C:
			actuales, err := s.logLines(r.Context(), actor, id, lineasIniciales)
			if err != nil {
				emitirEvento(w, "error", "Se perdio el contacto con el servidor.")
				flusher.Flush()
				return
			}

			for _, l := range lineasNuevas(previas, actuales) {
				emitirEvento(w, "linea", l)
			}
			// Un comentario SSE mantiene viva la conexion aunque no haya nada
			// que contar: sin trafico, un proxy intermedio la cierra por
			// inactividad y el directo se corta cuando el servidor esta
			// tranquilo, que es justo cuando parece que "no funciona".
			fmt.Fprint(w, ": latido\n\n")
			flusher.Flush()

			previas = actuales
		}
	}
}

// logLines lee el log y lo parte en lineas, sin las vacias del final.
func (s *Server) logLines(ctx context.Context, actor *domain.User, id int64, n int) ([]string, error) {
	texto, err := s.instances.Logs(ctx, actor, id, n)
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimRight(texto, "\n"), "\n"), nil
}

// lineasNuevas averigua que se ha escrito desde la ultima vuelta.
//
// No basta con contar: la ventana se desliza. Con tail=200, cuando el servidor
// escribe tres lineas nuevas, las tres mas viejas desaparecen por delante, asi
// que las dos listas siguen midiendo lo mismo aunque el contenido haya
// cambiado.
//
// Las dos son ventanas solapadas del mismo flujo. Se busca el solape MAS
// GRANDE posible: se prueba suponiendo cero lineas nuevas, luego una, luego
// dos... y se acepta la primera suposicion en la que lo que deberia ser comun
// encaja de verdad con el final de la vuelta anterior.
//
// Se busca el solape mayor y no el primero que cuadre porque un log de
// Minecraft repite lineas identicas sin parar -"Running AutoCompaction..."-;
// aceptar un solape corto haria coincidir con la repeticion equivocada y
// reenviaria historia vieja como si fuera nueva.
//
// El caso de solape cero siempre encaja, asi que esto siempre termina: si el
// servidor escribio mas de una ventana entera entre dos vueltas, o se
// reinicio, se devuelve todo. Repetir lineas es molesto; perderlas es peor.
func lineasNuevas(previas, actuales []string) []string {
	if len(previas) == 0 {
		return actuales
	}

	for nuevas := 0; nuevas <= len(actuales); nuevas++ {
		comun := actuales[:len(actuales)-nuevas]
		if len(comun) > len(previas) {
			continue // no puede ser el final de algo mas corto
		}
		if coinciden(previas[len(previas)-len(comun):], comun) {
			return actuales[len(actuales)-nuevas:]
		}
	}
	return actuales
}

func coinciden(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// emitirEvento escribe un evento SSE.
//
// Cada linea del mensaje va con su propio "data:", que es lo que exige el
// formato: un salto de linea suelto dentro del valor terminaria el evento
// antes de tiempo y partiria el mensaje en dos.
func emitirEvento(w http.ResponseWriter, nombre, mensaje string) {
	fmt.Fprintf(w, "event: %s\n", nombre)
	for _, l := range strings.Split(mensaje, "\n") {
		fmt.Fprintf(w, "data: %s\n", l)
	}
	fmt.Fprint(w, "\n")
}
