package web

import (
	"net/http"
	"strconv"

	"github.com/Excelion29/mc-config/internal/domain"
)

func (s *Server) showRoles(w http.ResponseWriter, r *http.Request) {
	s.renderRoles(w, r, http.StatusOK, "", "")
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	level, err := strconv.Atoi(r.PostFormValue("level"))
	if err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "Indica un nivel valido.", "")
		return
	}

	role, err := s.auth.CreateRole(r.Context(), actor,
		r.PostFormValue("code"), r.PostFormValue("name"), level,
		permissionsFromForm(r), clientIP(r))
	if err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}

	s.renderRoles(w, r, http.StatusOK, "", "Rol "+role.Name+" creado.")
}

func (s *Server) setRolePermissions(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "Rol invalido.", "")
		return
	}

	if err := s.auth.SetRolePermissions(r.Context(), actor, id,
		permissionsFromForm(r), clientIP(r)); err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}

	s.renderRoles(w, r, http.StatusOK, "", "Permisos guardados.")
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	actor := userFrom(r)

	if err := r.ParseForm(); err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "No se pudo leer el formulario.", "")
		return
	}

	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, "Rol invalido.", "")
		return
	}

	if err := s.auth.DeleteRole(r.Context(), actor, id, clientIP(r)); err != nil {
		s.renderRoles(w, r, http.StatusBadRequest, s.errorMessage(err), "")
		return
	}

	s.renderRoles(w, r, http.StatusOK, "", "Rol borrado.")
}

func (s *Server) renderRoles(w http.ResponseWriter, r *http.Request, status int, errMsg, infoMsg string) {
	actor := userFrom(r)

	roles, err := s.auth.ListRoles(r.Context(), actor)
	if err != nil {
		s.renderFailure(w, actor, "Roles", "No se pudo leer la lista de roles.", err)
		return
	}

	// Se cuenta cuanta gente usa cada rol para poder avisar antes de borrarlo.
	counts := make(map[int64]int, len(roles))
	if users, err := s.auth.ListUsers(r.Context(), actor); err == nil {
		for _, u := range users {
			counts[u.RoleID]++
		}
	}

	s.renderer.render(w, status, "roles.html", rolesPageData{
		PageData:    PageData{Title: "Roles", User: actor, Error: errMsg, Info: infoMsg},
		Roles:       roles,
		Groups:      groupPermissions(),
		UsersByRole: counts,
	})
}

// permissionsFromForm lee las casillas marcadas. La validacion contra el
// catalogo la hace el caso de uso: aqui solo se recoge.
func permissionsFromForm(r *http.Request) []domain.Permission {
	raw := r.PostForm["permissions"]
	perms := make([]domain.Permission, 0, len(raw))
	for _, code := range raw {
		perms = append(perms, domain.Permission(code))
	}
	return perms
}

// permissionGroup es el catalogo agrupado para pintarlo por secciones.
type permissionGroup struct {
	Name        string
	Permissions []domain.PermissionDef
}

func groupPermissions() []permissionGroup {
	var groups []permissionGroup
	for _, def := range domain.Permissions {
		if n := len(groups); n > 0 && groups[n-1].Name == def.Group {
			groups[n-1].Permissions = append(groups[n-1].Permissions, def)
			continue
		}
		groups = append(groups, permissionGroup{
			Name:        def.Group,
			Permissions: []domain.PermissionDef{def},
		})
	}
	return groups
}
