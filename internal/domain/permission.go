package domain

import "sort"

// Permission es una capacidad concreta del panel.
//
// El CATALOGO de permisos vive en el codigo, no en la base de datos, y es a
// proposito: un permiso solo existe si algun handler lo comprueba. Poder
// inventarlos desde la interfaz seria fingir un control que no existe.
//
// Lo que si son datos editables son los ROLES: que permisos agrupa cada uno.
type Permission string

const (
	PermServerView    Permission = "server.view"
	PermServerOperate Permission = "server.operate"

	// PermWorldImport cubre anadir mundos, tanto importando un mapa como
	// creando uno vacio. El codigo sigue siendo "map.import" porque esta
	// guardado en la base: renombrarlo obligaria a migrar los permisos ya
	// concedidos, y el codigo no lo lee nadie de fuera.
	PermWorldImport Permission = "map.import"
	PermWorldDelete Permission = "map.delete"

	PermInstanceCreate Permission = "instance.create"
	PermInstanceDelete Permission = "instance.delete"

	PermPlayerManage Permission = "player.manage"

	PermUserManage Permission = "user.manage"
	PermRoleManage Permission = "role.manage"
	PermAuditView  Permission = "audit.view"
)

// PermissionDef describe un permiso para poder pintarlo en la interfaz.
type PermissionDef struct {
	Code        Permission
	Group       string // agrupacion visual
	Label       string
	Description string
	// Destructive marca los permisos que pueden perder datos. Por D-08 no
	// deberian darse a un operador a la ligera; la interfaz los senala.
	Destructive bool
}

// Permissions es el catalogo completo. El orden es el de la pantalla de roles.
var Permissions = []PermissionDef{
	{PermServerView, "Servidores", "Ver estado",
		"Ver si hay un servidor encendido, que mapa tiene y quien esta conectado.", false},
	{PermServerOperate, "Servidores", "Arrancar, parar y cambiar de mapa",
		"Cambiar de mapa desconecta a quien este jugando (D-02).", false},

	{PermWorldImport, "Mundos", "Anadir mundos",
		"Subir archivos .mcworld y anadirlos a la biblioteca.", false},
	{PermWorldDelete, "Mundos", "Borrar mundos",
		"Elimina el mapa de la biblioteca y del disco.", true},

	{PermInstanceCreate, "Instancias", "Crear instancias",
		"Preparar un servidor con una version y un mapa concretos.", false},
	{PermInstanceDelete, "Instancias", "Borrar instancias",
		"Elimina el mundo y todo lo jugado en el.", true},

	{PermPlayerManage, "Jugadores", "Gestionar jugadores",
		"Anadir y quitar gamertags de la lista maestra, y marcar operadores (D-13).", false},

	{PermUserManage, "Panel", "Gestionar usuarios del panel",
		"Crear cuentas, cambiar su rol y activarlas o desactivarlas.", false},
	{PermRoleManage, "Panel", "Gestionar roles y permisos",
		"Crear roles y decidir que puede hacer cada uno.", false},
	{PermAuditView, "Panel", "Ver el registro de acciones",
		"Consultar quien hizo que y cuando (D-08).", false},
}

func PermissionByCode(code Permission) (PermissionDef, bool) {
	for _, d := range Permissions {
		if d.Code == code {
			return d, true
		}
	}
	return PermissionDef{}, false
}

// AllPermissions devuelve todos los codigos del catalogo.
func AllPermissions() []Permission {
	out := make([]Permission, 0, len(Permissions))
	for _, d := range Permissions {
		out = append(out, d.Code)
	}
	return out
}

// PermissionSet es un conjunto de permisos.
type PermissionSet map[Permission]struct{}

func NewPermissionSet(perms ...Permission) PermissionSet {
	s := make(PermissionSet, len(perms))
	for _, p := range perms {
		s[p] = struct{}{}
	}
	return s
}

func (s PermissionSet) Has(p Permission) bool {
	_, ok := s[p]
	return ok
}

// List devuelve los permisos ordenados, para que la salida sea estable.
func (s PermissionSet) List() []Permission {
	out := make([]Permission, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
