package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/domain"
)

func (s *Server) showUsers(w http.ResponseWriter, r *http.Request) {
	s.renderUsers(w, r, http.StatusOK, "", "")
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	roleID, err := strconv.ParseInt(r.PostFormValue("role_id"), 10, 64)
	if err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "Selecciona un rol valido.", "")
		return
	}

	u, err := s.auth.AddUser(r.Context(), actor,
		r.PostFormValue("email"), r.PostFormValue("password"), roleID, clientIP(r))
	if err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}

	s.renderUsers(w, r, http.StatusOK, "",
		"Usuario "+u.Email+" creado como "+u.RoleName()+".")
}

func (s *Server) setUserActive(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "Usuario invalido.", "")
		return
	}
	active := r.PostFormValue("active") == "1"

	if err := s.auth.SetUserActive(r.Context(), actor, id, active, clientIP(r)); err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}

	msg := "Usuario desactivado."
	if active {
		msg = "Usuario activado."
	}
	s.renderUsers(w, r, http.StatusOK, "", msg)
}

func (s *Server) setUserRole(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	id, err1 := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	roleID, err2 := strconv.ParseInt(r.PostFormValue("role_id"), 10, 64)
	if err1 != nil || err2 != nil {
		s.renderUsers(w, r, http.StatusBadRequest, "Datos invalidos.", "")
		return
	}

	if err := s.auth.SetUserRole(r.Context(), actor, id, roleID, clientIP(r)); err != nil {
		s.renderUsers(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}
	s.renderUsers(w, r, http.StatusOK, "", "Rol actualizado.")
}

// renderUsers recarga siempre la lista desde la base antes de mostrarla, para
// que la pantalla refleje el estado real y no lo que creiamos que era.
func (s *Server) renderUsers(w http.ResponseWriter, r *http.Request, status int, errMsg, infoMsg string) {
	actor := userFrom(r)

	users, err := s.auth.ListUsers(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Usuarios", "No se pudo leer la lista de usuarios.", err)
		return
	}
	roles, err := s.auth.RolesForAssignment(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Usuarios", "No se pudo leer la lista de roles.", err)
		return
	}

	s.renderer.render(w, status, "users.html", usersPageData{
		PageData: PageData{Title: "Usuarios", User: actor, Error: errMsg, Info: infoMsg},
		Users:    users,
		Roles:    roles,
	})
}

func (s *Server) renderFailure(w http.ResponseWriter, actor *domain.User, title, msg string, err error) {
	s.log.Error("fallo mostrando una pagina", "pagina", title, "error", err)
	s.renderer.render(w, http.StatusInternalServerError, "error.html", PageData{
		Title: title,
		User:  actor,
		Error: msg,
	})
}

// errorMessage traduce errores del dominio a texto para la persona. Cualquier
// error inesperado se registra y se muestra generico: los detalles internos no
// deben acabar en pantalla.
func (s *Server) errorMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrDuplicateEmail):
		return "Ya existe un usuario con ese correo."
	case errors.Is(err, domain.ErrEmptyEmail):
		return "El correo es obligatorio."
	case errors.Is(err, domain.ErrPasswordTooShort):
		return "La contrasena debe tener al menos 8 caracteres."
	case errors.Is(err, domain.ErrSelfDisable):
		return "No puedes desactivar tu propia cuenta."
	case errors.Is(err, domain.ErrSelfRoleDowngrade):
		return "No puedes quitarte a ti mismo la gestion de usuarios: te quedarias sin poder arreglarlo."
	case errors.Is(err, domain.ErrForbidden):
		return "No tienes permiso para esta accion."
	case errors.Is(err, domain.ErrNotFound):
		return "Ese usuario ya no existe."
	case errors.Is(err, domain.ErrRoleNotFound):
		return "Ese rol ya no existe."
	case errors.Is(err, domain.ErrDuplicateRole):
		return "Ya existe un rol con ese codigo."
	case errors.Is(err, domain.ErrEmptyRoleCode):
		return "El codigo del rol es obligatorio."
	case errors.Is(err, domain.ErrSystemRole):
		return "Los roles del sistema no se pueden borrar."
	case errors.Is(err, domain.ErrSuperuserLocked):
		return "El superusuario no lo puede gestionar nadie, y su rol no se edita."
	case errors.Is(err, domain.ErrSamePeer):
		return "No puedes gestionar a alguien de tu mismo nivel."
	case errors.Is(err, domain.ErrRoleAboveYou):
		return "No puedes asignar ni editar un rol igual o superior al tuyo."
	case errors.Is(err, domain.ErrRoleLevelTooHigh):
		return "El nivel del rol debe estar por debajo del tuyo."
	case errors.Is(err, domain.ErrOnlyOneSuperuser):
		return "Solo puede haber un superusuario, y no se asigna desde aqui."
	case errors.Is(err, domain.ErrAdminRoleLocked):
		return "Ese rol tiene siempre todos los permisos."
	case errors.Is(err, domain.ErrRoleInUse):
		return "Hay usuarios con ese rol. Cambialos de rol antes de borrarlo."
	case errors.Is(err, domain.ErrUnknownPermiso):
		return "Se recibio un permiso que no existe."
	default:
		s.log.Error("error inesperado", "error", err)
		return "Ha ocurrido un error. Intentalo de nuevo."
	}
}
