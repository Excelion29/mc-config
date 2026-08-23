// Package domain contiene las entidades y las reglas de negocio del panel.
//
// Regla del paquete: NO importa nada de infraestructura (ni SQL, ni HTTP, ni
// Docker). Solo la libreria estandar. Asi las reglas se pueden probar sin
// levantar una base de datos, y cambiar de SQLite a PostgreSQL (D-14) no toca
// una sola linea de aqui.
package domain

import "time"

// User es una cuenta del panel. No es un jugador de Minecraft: son dos
// identidades distintas y no se mezclan (ver D-13).
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	RoleID       int64
	// Role viene cargado con sus permisos. Puede ser nil si se leyo el usuario
	// sin resolver el rol; Can() lo trata como "no puede nada".
	Role      *Role
	Active    bool
	CreatedAt time.Time
}

// Can es el unico sitio donde se decide si alguien puede hacer algo (D-08).
func (u *User) Can(p Permission) bool {
	if u == nil || !u.Active {
		return false
	}
	return u.Role.Can(p)
}

// CanManage decide si este usuario puede gestionar al otro.
//
// Tres reglas, en este orden:
//  1. Al superusuario no lo gestiona nadie, ni siquiera otro superusuario.
//  2. Nadie se gestiona a si mismo desde la pantalla de usuarios; para eso
//     estan las guardas de autodesactivacion.
//  3. Solo se gestiona a quien esta estrictamente por debajo en la jerarquia.
func (u *User) CanManage(target *User) bool {
	if u == nil || target == nil {
		return false
	}
	if target.Role.IsSuperuser() {
		return false
	}
	if u.ID == target.ID {
		return false
	}
	return u.Role.Outranks(target.Role)
}

// RoleLevel expone el nivel para las plantillas.
func (u *User) RoleLevel() int {
	if u == nil || u.Role == nil {
		return 0
	}
	return u.Role.Level
}

// RoleName es el nombre visible del rol, para plantillas.
func (u *User) RoleName() string {
	if u == nil || u.Role == nil {
		return "sin rol"
	}
	return u.Role.Name
}

// RoleCode se usa en las clases CSS.
func (u *User) RoleCode() string {
	if u == nil || u.Role == nil {
		return ""
	}
	return u.Role.Code
}

// MinPasswordLength es la unica regla de contrasena que impone el dominio.
const MinPasswordLength = 8
