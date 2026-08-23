package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// Todas las acciones siguen POST/Redirect/GET: nunca pintan la pagina como
// respuesta a un POST. Asi recargar no reenvia el formulario, y el auto-refresco
// de la pantalla no acaba lanzando un GET contra una ruta que solo acepta POST
// (que fue justo el 405 que aparecio al probarlo).
const rutaInstancias = "/instances"

func (s *Server) showInstances(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	info, errMsg := s.takeFlash(w, r)

	list, err := s.instances.List(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Servidores", "No se pudo leer la lista de servidores.", err)
		return
	}

	// El estado real se consulta a Docker: un contenedor pudo caerse por su
	// cuenta, o seguir descargando el binario, y la fila no lo sabria.
	var online, max int
	for i := range list {
		if list[i].State == domain.StateRunning || list[i].State == domain.StateStarting {
			if inst, o, m, err := s.instances.Status(r.Context(), actor, list[i].ID); err == nil {
				list[i] = *inst
				if inst.State == domain.StateRunning {
					online, max = o, m
				}
			}
		}
	}

	maps, err := s.maps.List(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Servidores", "No se pudo leer la biblioteca de mapas.", err)
		return
	}

	// Si la consulta falla, la lista queda vacia y la plantilla cae al campo
	// libre: no impedir crear servidores porque Mojang no responda.
	versions, _ := s.instances.Versions(r.Context(), actor, domain.EditionBedrock)

	// La confirmacion viaja por la URL para sobrevivir a la redireccion.
	var confirm *confirmSwitch
	if raw := r.URL.Query().Get("confirmar"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if running, players, err := s.instances.SwitchPreview(r.Context(), actor, id); err == nil && running != nil {
				confirm = &confirmSwitch{TargetID: id, Running: running, Players: players}
			}
		}
	}

	// Arrancar puede tardar minutos la primera vez, porque la imagen descarga
	// el binario del servidor. Mientras tanto la pagina se refresca sola.
	// De paso se localiza la instancia que ocupa el turno. Como solo puede
	// haber una encendida, una que este arrancando o parando tambien cuenta:
	// es la que tiene el puerto y la que le importa a quien mira la pantalla.
	transicion := false
	var current *domain.Instance
	for i := range list {
		inst := &list[i]
		if inst.State.Busy() {
			transicion = true
		}
		if current == nil && (inst.State == domain.StateRunning || inst.State.Busy()) {
			current = inst
		}
	}

	page := s.pagina(r, "Servidores", errMsg, info)
	if transicion && confirm == nil {
		page.Refresh = 5
	}

	s.renderer.render(w, http.StatusOK, "instances.html", instancesPageData{
		PageData:     page,
		EnTransicion: transicion,
		Instances:    list,
		Current:      current,
		Maps:         maps,
		Online:       online,
		MaxOnline:    max,
		Versions:     versions,
		Confirm:      confirm,
	})
}

func (s *Server) createInstance(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaInstancias, "No se pudo leer el formulario.")
		return
	}

	mapID, err := strconv.ParseInt(r.PostFormValue("map_id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaInstancias, "Selecciona un mapa.")
		return
	}

	// El desplegable manda, salvo que se haya elegido "Otra": entonces vale lo
	// escrito a mano.
	version := r.PostFormValue("version")
	if version == "" {
		version = r.PostFormValue("version_manual")
	}

	inst, err := s.instances.Create(r.Context(), actor,
		r.PostFormValue("name"), mapID, version, nil, clientIP(r))
	if err != nil {
		s.redirectError(w, r, rutaInstancias, s.instanceError(err))
		return
	}

	s.redirectInfo(w, r, rutaInstancias,
		"Instancia \""+inst.Name+"\" creada. Ya puedes arrancarla.")
}

func (s *Server) startInstance(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaInstancias, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaInstancias, "Instancia invalida.")
		return
	}
	confirmed := r.PostFormValue("confirm") == "1"

	err = s.instances.Start(r.Context(), actor, id, confirmed, clientIP(r))

	// Por D-02 solo hay un servidor encendido: arrancar otro desconecta a quien
	// este jugando. No se hace sin que alguien lo confirme viendo a cuanta
	// gente afecta (D-08).
	var needs *app.NeedsConfirmation
	if errors.As(err, &needs) {
		http.Redirect(w, r, rutaInstancias+"?confirmar="+strconv.FormatInt(id, 10), http.StatusSeeOther)
		return
	}
	if err != nil {
		s.redirectError(w, r, rutaInstancias, s.instanceError(err))
		return
	}

	s.redirectInfo(w, r, rutaInstancias,
		"Arrancando. La primera vez tarda unos minutos: descarga el servidor de Mojang.")
}

func (s *Server) stopInstance(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaInstancias, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaInstancias, "Instancia invalida.")
		return
	}

	if err := s.instances.Stop(r.Context(), actor, id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaInstancias, s.instanceError(err))
		return
	}
	s.redirectInfo(w, r, rutaInstancias, "Servidor detenido limpiamente.")
}

func (s *Server) deleteInstance(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, rutaInstancias, "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, rutaInstancias, "Instancia invalida.")
		return
	}

	if err := s.instances.Delete(r.Context(), actor, id, clientIP(r)); err != nil {
		s.redirectError(w, r, rutaInstancias, s.instanceError(err))
		return
	}
	if esParcial(r) {
		// Cuerpo vacio: la fila se quita, no se sustituye.
		//
		// Arrancar y parar NO usan esto a proposito: cambian el panel de "en
		// este momento" y el resto de la pantalla, no solo una fila. Ahi la
		// recarga completa dice la verdad y un intercambio de fila dejaria
		// medio panel mintiendo.
		w.WriteHeader(http.StatusOK)
		return
	}
	s.redirectInfo(w, r, rutaInstancias, "Instancia borrada.")
}

func (s *Server) instanceLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chiParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	logs, err := s.instances.Logs(r.Context(), userFrom(r), id, 200)
	if err != nil {
		http.Error(w, "no se pudieron leer los logs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(logs))
}

// confirmSwitch son los datos del aviso antes de cambiar de servidor.
type confirmSwitch struct {
	TargetID int64
	Running  *domain.Instance
	Players  int
}

func (s *Server) instanceError(err error) string {
	switch {
	case errors.Is(err, domain.ErrInstanceNotFound):
		return "Esa instancia ya no existe."
	case errors.Is(err, domain.ErrDuplicateInstance):
		return "Ya existe una instancia con ese nombre."
	case errors.Is(err, domain.ErrEmptyInstanceName):
		return "El nombre es obligatorio."
	case errors.Is(err, domain.ErrInstanceBusy):
		return "La instancia esta arrancando o parando. Espera a que termine."
	case errors.Is(err, domain.ErrInstanceRunning):
		return "Detén la instancia antes de borrarla."
	case errors.Is(err, domain.ErrEditionMismatch):
		return "La edicion del mapa no coincide con la del servidor."
	case errors.Is(err, domain.ErrMapNotFound):
		return "Ese mapa ya no esta en la biblioteca."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con una instancia", "error", err)
		return "Ha ocurrido un error: " + err.Error()
	}
}
