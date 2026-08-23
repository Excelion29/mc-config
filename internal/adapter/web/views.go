package web

import (
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Tipos de vista: lo que recibe una plantilla, ya masticado.
//
// Antes las plantillas recibian domain.User y domain.Role directamente y
// resolvian ahi las decisiones -"puede este usuario gestionar a este otro?"-
// llamando a metodos del dominio dentro del HTML. Eso trae dos problemas:
//
//   - Una regla de negocio evaluada en una plantilla no la comprueba nadie.
//     Renombrar un metodo del dominio no rompe la compilacion, rompe la
//     pagina, y solo se descubre al abrirla.
//
//   - La misma condicion acababa escrita en dos sitios. En roles.html decidia
//     si pintar el boton de editar Y, sesenta lineas mas abajo, si generar el
//     dialogo que ese boton abre. Si una cambia y la otra no, queda un boton
//     que no abre nada.
//
// Con estos tipos la decision se toma una vez, en Go, y la plantilla solo
// pregunta por un booleano.

// userView es una fila de la tabla de usuarios.
type userView struct {
	ID        int64
	Email     string
	RoleID    int64
	RoleName  string
	RoleLevel int
	Active    bool
	CreatedAt time.Time

	// EsTuCuenta distingue la fila del propio actor: no se gestiona a uno
	// mismo, pero tampoco es "bloqueado", es otra cosa.
	EsTuCuenta bool
	// Gestionable resume la jerarquia completa: solo se toca a quien esta
	// estrictamente por debajo, nunca al superusuario, nunca a uno mismo.
	Gestionable bool
}

func nuevaVistaUsuarios(actor *domain.User, usuarios []domain.User) []userView {
	vistas := make([]userView, 0, len(usuarios))
	for i := range usuarios {
		u := &usuarios[i]
		vistas = append(vistas, userView{
			ID:          u.ID,
			Email:       u.Email,
			RoleID:      u.RoleID,
			RoleName:    u.RoleName(),
			RoleLevel:   u.RoleLevel(),
			Active:      u.Active,
			CreatedAt:   u.CreatedAt,
			EsTuCuenta:  actor != nil && actor.ID == u.ID,
			Gestionable: actor.CanManage(u),
		})
	}
	return vistas
}

// roleView es una fila de la tabla de roles.
type roleView struct {
	ID     int64
	Code   string
	Name   string
	Level  int
	System bool

	// TodosLosPermisos evita que la plantilla tenga que saber que el
	// superusuario es un caso especial: aqui ya es un si o un no.
	TodosLosPermisos bool
	Permisos         int
	Usuarios         int

	// Editable es la condicion que antes estaba duplicada: gobierna a la vez
	// el boton y el dialogo que abre, asi que no pueden discrepar.
	Editable bool
	// Borrable ademas exige que no sea un rol del sistema.
	Borrable bool
}

func nuevaVistaRoles(actor *domain.User, roles []domain.Role, usuariosPorRol map[int64]int) []roleView {
	vistas := make([]roleView, 0, len(roles))
	for i := range roles {
		r := &roles[i]
		editable := !r.IsSuperuser() && actor != nil && actor.Role.Outranks(r)
		vistas = append(vistas, roleView{
			ID:               r.ID,
			Code:             r.Code,
			Name:             r.Name,
			Level:            r.Level,
			System:           r.System,
			TodosLosPermisos: r.IsSuperuser(),
			Permisos:         len(r.Permissions),
			Usuarios:         usuariosPorRol[r.ID],
			Editable:         editable,
			Borrable:         editable && !r.System,
		})
	}
	return vistas
}

// permisosPorRol indexa los permisos por id, para que el dialogo de edicion
// los consulte sin recorrer la lista en cada iteracion.
//
// Devuelve mapas de BOOLEANOS y no el domain.PermissionSet tal cual, que es un
// map[Permission]struct{}. En una plantilla, "index" sobre ese conjunto
// devuelve struct{}{} para las claves que faltan, y una estructura vacia
// siempre es verdadera para {{if}}: saldrian todos los permisos marcados, en
// todos los roles, sin ningun error a la vista.
func permisosPorRol(roles []domain.Role) map[int64]map[domain.Permission]bool {
	m := make(map[int64]map[domain.Permission]bool, len(roles))
	for i := range roles {
		marcados := make(map[domain.Permission]bool, len(roles[i].Permissions))
		for p := range roles[i].Permissions {
			marcados[p] = true
		}
		m[roles[i].ID] = marcados
	}
	return m
}
