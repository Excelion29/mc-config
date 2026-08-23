package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

func (s *Server) showMaps(w http.ResponseWriter, r *http.Request) {
	s.renderMaps(w, r, "", "")
}

func (s *Server) importMap(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)
	max := s.maps.MaxUpload()

	// Se limita el cuerpo ANTES de leerlo: sin esto, un archivo enorme se
	// escribe entero en disco antes de que nadie compruebe su tamano.
	r.Body = http.MaxBytesReader(w, r.Body, max+1<<20)

	// El primer argumento es cuanto se mantiene en memoria; el resto va a
	// temporales del sistema. Con 16 MiB los formularios normales no tocan
	// disco y los mapas grandes no se cargan en RAM.
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		s.redirectError(w, r, "/maps",
			"No se pudo leer el archivo. Puede que supere el tamano maximo ("+
				app.HumanSize(max)+").")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("archivo")
	if err != nil {
		s.redirectError(w, r, "/maps", "Selecciona un archivo .mcworld o .zip.")
		return
	}
	defer file.Close()

	mp, err := s.maps.Import(r.Context(), actor, file, header.Filename, header.Size, clientIP(r))
	if err != nil {
		// El codigo de estado deja de importar: la respuesta pasa a ser
		// siempre una redireccion y el motivo viaja en el flash.
		s.redirectError(w, r, "/maps", s.mapErrorMessage(err, mp))
		return
	}

	s.redirectInfo(w, r, "/maps",
		"Mapa \""+mp.Name+"\" importado: "+mp.Edition.Label()+" "+mp.Version+".")
}

func (s *Server) deleteMap(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.redirectError(w, r, "/maps", "No se pudo leer el formulario.")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.redirectError(w, r, "/maps", "Mapa invalido.")
		return
	}

	if err := s.maps.Delete(r.Context(), actor, id, clientIP(r)); err != nil {
		s.redirectError(w, r, "/maps", s.mapErrorMessage(err, nil))
		return
	}
	s.redirectInfo(w, r, "/maps", "Mapa borrado.")
}

// mapIcon sirve la miniatura que venia dentro del .mcworld (H-F0-4).
func (s *Server) mapIcon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chiParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data, err := s.maps.Icon(r.Context(), userFrom(r), id)
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(data)
}

func (s *Server) renderMaps(w http.ResponseWriter, r *http.Request, errMsg, infoMsg string) {
	// El mensaje de la accion anterior llega por el flash de la redireccion:
	// se muestra una vez y se borra. Antes se renderizaba directamente desde
	// el POST, y refrescar reenviaba el formulario.
	if info, err := s.takeFlash(w, r); info != "" || err != "" {
		infoMsg, errMsg = info, err
	}

	actor := userFrom(r)

	pagina, _ := strconv.Atoi(r.URL.Query().Get("p"))
	page, err := s.maps.ListPage(r.Context(), actor, app.Paging{Page: pagina})
	if err != nil {
		s.renderFailure(w, actor, "Mapas", "No se pudo leer la biblioteca de mapas.", err)
		return
	}

	s.renderer.render(w, http.StatusOK, "maps.html", mapsPageData{
		PageData:  s.pagina(r, "Mapas", errMsg, infoMsg),
		Maps:      page.Maps,
		MaxUpload: app.HumanSize(s.maps.MaxUpload()),
		Pag:       paginador{Info: page.PageInfo, Base: "/maps?"},
	})
}

// mapErrorMessage explica el fallo en terminos que sirvan para arreglarlo.
func (s *Server) mapErrorMessage(err error, dup *domain.Map) string {
	switch {
	case errors.Is(err, domain.ErrDuplicateMap):
		if dup != nil {
			return "Ese archivo ya esta en la biblioteca como \"" + dup.Name + "\"."
		}
		return "Ese mapa ya esta en la biblioteca."
	case errors.Is(err, domain.ErrNoFile):
		return "No se recibio ningun archivo."
	case errors.Is(err, domain.ErrFileTooBig):
		return "El archivo supera el tamano maximo (" + app.HumanSize(s.maps.MaxUpload()) + ")."
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
	case errors.Is(err, domain.ErrMapNotFound):
		return "Ese mapa ya no existe."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	default:
		s.log.Error("error inesperado con un mapa", "error", err)
		return "Ha ocurrido un error al procesar el archivo."
	}
}
