package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

func (s *Server) showWorlds(w http.ResponseWriter, r *http.Request) {
	s.renderWorlds(w, r, "", "")
}

// createWorld da de alta un mundo vacio, que generara el servidor al arrancar.
//
// No comprueba el disco a proposito: un mundo creado no ocupa nada hasta que se
// enciende, y quien lo enciende ya lo comprueba. Bloquearlo aqui impediria
// preparar un mundo estando justo de espacio, que es cuando mas falta hace
// poder organizarse.
// versionesDe consulta las versiones instalables de una edicion.
func (s *Server) versionesDe(r *http.Request, actor *domain.User, e domain.Edition) []grupoVersiones {
	v, err := s.instances.Versions(r.Context(), actor, e)
	if err != nil {
		return nil
	}
	return agruparVersiones(v)
}

func (s *Server) createWorld(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	maxJug, err := strconv.Atoi(r.PostFormValue("max_players"))
	if err != nil {
		s.redirectError(w, r, "/worlds", "Indica cuantos jugadores caben.")
		return
	}

	gen := domain.Generation{
		Seed:       r.PostFormValue("seed"),
		LevelType:  domain.LevelType(r.PostFormValue("level_type_" + r.PostFormValue("edition"))),
		Structures: r.PostFormValue("structures") == "1",
		BonusChest: r.PostFormValue("bonus_chest") == "1",
	}
	reglas := domain.Rules{
		Gamemode:      domain.Gamemode(r.PostFormValue("gamemode")),
		Difficulty:    domain.Difficulty(r.PostFormValue("difficulty")),
		AllowCommands: r.PostFormValue("allow_commands") == "1",
		PvP:           r.PostFormValue("pvp") == "1",
		MaxPlayers:    maxJug,
	}

	// Se mandan los dos desplegables y aqui se coge el de la edicion elegida.
	// Es la contrapartida de resolverlo con CSS en vez de JavaScript: el
	// oculto viaja igual, asi que hay que ignorarlo a proposito.
	edicion := domain.Edition(r.PostFormValue("edition"))
	version := r.PostFormValue("version_" + string(edicion))

	mundo, err := s.worlds.Create(r.Context(), actor,
		r.PostFormValue("name"), edicion, version, r.PostFormValue("icon_url"),
		gen, reglas, clientIP(r))
	if err != nil {
		s.redirectError(w, r, "/worlds", s.worldErrorMessage(err, nil))
		return
	}

	s.redirectInfo(w, r, "/worlds",
		"Mundo \""+mundo.Name+"\" creado. El terreno se genera la primera vez que arranques un servidor con el.")
}

// updateWorld cambia lo que SI se puede cambiar de un mundo.
//
// Nombre, portada y reglas. La generacion no: da forma al terreno una sola vez
// y cambiarla despues no reescribe lo que ya hay en disco.
func (s *Server) updateWorld(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/worlds", "Mundo invalido.")
		return
	}
	maxJug, err := strconv.Atoi(r.PostFormValue("max_players"))
	if err != nil {
		s.redirectError(w, r, "/worlds", "Indica cuantos jugadores caben.")
		return
	}

	mundo, err := s.worlds.Update(r.Context(), actor, id,
		r.PostFormValue("name"), r.PostFormValue("icon_url"),
		domain.Rules{
			Gamemode:      domain.Gamemode(r.PostFormValue("gamemode")),
			Difficulty:    domain.Difficulty(r.PostFormValue("difficulty")),
			AllowCommands: r.PostFormValue("allow_commands") == "1",
			PvP:           r.PostFormValue("pvp") == "1",
			MaxPlayers:    maxJug,
		}, clientIP(r))
	if err != nil {
		s.redirectError(w, r, "/worlds", s.worldErrorMessage(err, nil))
		return
	}

	s.redirectInfo(w, r, "/worlds",
		"Mundo \""+mundo.Name+"\" actualizado. Las reglas se aplican la proxima vez que arranques un servidor con el.")
}

func (s *Server) importWorld(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	max := s.worlds.MaxUpload()

	// Se limita el cuerpo ANTES de leerlo: sin esto, un archivo enorme se
	// escribe entero en disco antes de que nadie compruebe su tamano.
	r.Body = http.MaxBytesReader(w, r.Body, max+1<<20)

	// El primer argumento es cuanto se mantiene en memoria; el resto va a
	// temporales del sistema. Con 16 MiB los formularios normales no tocan
	// disco y los mapas grandes no se cargan en RAM.
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		s.redirectError(w, r, "/worlds",
			"No se pudo leer el archivo. Puede que supere el tamano maximo ("+
				app.HumanSize(max)+").")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("archivo")
	if err != nil {
		s.redirectError(w, r, "/worlds", "Selecciona un archivo .mcworld o .zip.")
		return
	}
	defer file.Close()

	mp, err := s.worlds.Import(r.Context(), actor, file, header.Filename, header.Size, clientIP(r))
	if err != nil {
		// El codigo de estado deja de importar: la respuesta pasa a ser
		// siempre una redireccion y el motivo viaja en el flash.
		s.redirectError(w, r, "/worlds", s.worldErrorMessage(err, mp))
		return
	}

	s.redirectInfo(w, r, "/worlds",
		"Mapa \""+mp.Name+"\" importado: "+mp.Edition.Label()+" "+mp.Version+".")
}

func (s *Server) deleteWorld(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/worlds", "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/worlds", "Mapa invalido.")
		return
	}

	if err := s.worlds.Delete(r.Context(), actor, id, clientIP(r)); err != nil {
		s.redirectError(w, r, "/worlds", s.worldErrorMessage(err, nil))
		return
	}
	s.redirectInfo(w, r, "/worlds", "Mapa borrado.")
}

// worldIcon sirve la miniatura que venia dentro del .mcworld (H-F0-4).
func (s *Server) worldIcon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chiParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data, err := s.worlds.Icon(r.Context(), userFrom(r), id)
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(data)
}

func (s *Server) renderWorlds(w http.ResponseWriter, r *http.Request, errMsg, infoMsg string) {
	// El mensaje de la accion anterior llega por el flash de la redireccion:
	// se muestra una vez y se borra. Antes se renderizaba directamente desde
	// el POST, y refrescar reenviaba el formulario.
	if info, err := s.takeFlash(w, r); info != "" || err != "" {
		infoMsg, errMsg = info, err
	}

	actor := userFrom(r)

	pagina, _ := strconv.Atoi(r.URL.Query().Get("p"))
	page, err := s.worlds.ListPage(r.Context(), actor, app.Paging{Page: pagina})
	if err != nil {
		s.renderFailure(w, actor, "Mapas", "No se pudo leer la biblioteca de mapas.", err)
		return
	}

	s.renderer.render(w, http.StatusOK, "worlds.html", worldsPageData{
		PageData:  s.pagina(r, "Mapas", errMsg, infoMsg),
		Maps:      page.Maps,
		MaxUpload: app.HumanSize(s.worlds.MaxUpload()),
		// Por mundo, que recursos lleva. Si falla se pinta la pagina sin ellos:
		// no impedir gestionar mundos porque los recursos den problemas.
		RecursosPorMundo: s.recursosPorMundo(r, actor, page.Maps),
		// Si la consulta falla, la lista queda vacia y la plantilla cae al
		// campo libre: no impedir crear mundos porque un tercero no responda.
		VersionsBedrock: s.versionesDe(r, actor, domain.EditionBedrock),
		VersionsJava:    s.versionesDe(r, actor, domain.EditionJava),
		TypesBedrock:    domain.LevelTypesFor(domain.EditionBedrock),
		TypesJava:       domain.LevelTypesFor(domain.EditionJava),
		Pag:             paginador{Info: page.PageInfo, Base: "/maps?"},
	})
}

// worldErrorMessage explica el fallo en terminos que sirvan para arreglarlo.
func (s *Server) worldErrorMessage(err error, dup *domain.World) string {
	switch {
	case errors.Is(err, domain.ErrDuplicateWorld):
		if dup != nil {
			return "Ese archivo ya esta en la biblioteca como \"" + dup.Name + "\"."
		}
		return "Ese mapa ya esta en la biblioteca."
	case errors.Is(err, domain.ErrEmptyName):
		return "El nombre del mundo es obligatorio."
	case errors.Is(err, domain.ErrInvalidIconURL):
		return "La portada debe ser un enlace que empiece por https://"
	case errors.Is(err, domain.ErrInvalidSettings):
		return "Alguno de los ajustes no es valido."
	case errors.Is(err, domain.ErrNoFile):
		return "No se recibio ningun archivo."
	case errors.Is(err, domain.ErrFileTooBig):
		return "El archivo supera el tamano maximo (" + app.HumanSize(s.worlds.MaxUpload()) + ")."
	case errors.Is(err, domain.ErrNotAnArchive):
		return "El archivo no es un ZIP valido. Un .mcworld es un ZIP; comprueba que no se descargo a medias."
	case errors.Is(err, domain.ErrNotAWorld):
		return "El archivo no contiene un mundo: falta level.dat en su interior."
	case errors.Is(err, domain.ErrUnsafePath):
		return "El archivo contiene rutas que escaparian de su carpeta al extraerse. Rechazado."
	case errors.Is(err, domain.ErrZipBomb):
		return "El archivo se expande demasiado al descomprimirse. Rechazado por seguridad."
	case errors.Is(err, domain.ErrNoDiskSpace):
		return "No hay espacio suficiente en el disco del servidor."
	case errors.Is(err, domain.ErrWorldNotFound):
		return "Ese mapa ya no existe."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con un mapa", "error", err)
		return "Ha ocurrido un error al procesar el archivo."
	}
}

// recursosPorMundo resuelve, para cada mundo, que lleva y cual se aplica solo.
//
// Se hace aqui y no en la plantilla porque "esta este recurso en la lista de
// este mundo" es una busqueda, y una plantilla que busca acaba equivocandose en
// silencio.
func (s *Server) recursosPorMundo(r *http.Request, actor *domain.User, mundos []domain.World) map[int64]recursosDeMundo {
	out := make(map[int64]recursosDeMundo, len(mundos))

	for i := range mundos {
		m := &mundos[i]

		// Bedrock no sirve recursos por enlace, asi que el dialogo no aparece y
		// no hay nada que resolver (D-18).
		if m.Edition != domain.EditionJava {
			continue
		}

		datos, err := s.recursosDeMundo(r, actor, m)
		if err != nil {
			s.log.Warn("no se pudieron leer los recursos del mundo",
				"mundo", m.Name, "error", err)
			continue
		}
		out[m.ID] = datos
	}
	return out
}

// recursosDeUnMundo es lo mismo para un solo mundo, releyendolo de la base.
//
// Lo usa el repintado sin recargar: lo que vale es lo que quedo guardado, no lo
// que el navegador creia estar pidiendo.
func (s *Server) recursosDeUnMundo(r *http.Request, actor *domain.User, worldID int64) (recursosDeMundo, error) {
	mundo, err := s.worlds.ByID(r.Context(), actor, worldID)
	if err != nil {
		return recursosDeMundo{}, err
	}
	return s.recursosDeMundo(r, actor, mundo)
}

// recursosDeMundo resuelve que lleva un mundo y cual se aplica solo.
//
// Se hace aqui y no en la plantilla porque "esta este recurso en la lista de
// este mundo" es una busqueda, y una plantilla que busca acaba equivocandose en
// silencio.
func (s *Server) recursosDeMundo(r *http.Request, actor *domain.User, m *domain.World) (recursosDeMundo, error) {
	asignados, err := s.recursos.DeMundo(r.Context(), actor, m.ID)
	if err != nil {
		return recursosDeMundo{}, err
	}

	disponibles, err := s.recursos.Disponibles(r.Context(), actor, m.ID)
	if err != nil {
		s.log.Warn("no se pudo leer la biblioteca de recursos",
			"mundo", m.Name, "error", err)
	}

	datos := recursosDeMundo{
		WorldID:     m.ID,
		Nombre:      m.Name,
		Disponibles: vistasDeRecurso(disponibles),
		Requerido:   m.ResourceRequired,
		Hueco:       len(asignados) < domain.MaxRecursosPorMundo,
		Tope:        domain.MaxRecursosPorMundo,
	}
	for j := range asignados {
		a := &asignados[j]
		vista := vistaDeRecurso(&a.Resource)
		datos.Lista = append(datos.Lista, vista)
		if vista.Automatico {
			datos.HayAutomatico = true
		}
		if a.Principal {
			datos.Principal = a.ID
		}
	}
	return datos, nil
}
