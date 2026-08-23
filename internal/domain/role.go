package domain

import "time"

// Codigos de los roles que crea el sistema al arrancar.
const (
	RoleCodeSuperuser = "superuser"
	RoleCodeAdmin     = "admin"
	RoleCodeOperator  = "operator"
	RoleCodeViewer    = "viewer"
)

// Niveles de la jerarquia. Mas alto = mas poder.
//
// La regla es simple y se aplica en todas partes: solo se puede gestionar a
// quien esta ESTRICTAMENTE por debajo. Un administrador no toca a otro
// administrador.
//
// Sin alguien por encima de todos, dos administradores en desacuerdo dejarian
// el panel bloqueado: ninguno podria arreglar al otro. Para eso existe el
// superusuario, y por eso solo puede haber uno.
const (
	LevelSuperuser = 100
	LevelAdmin     = 50
	LevelOperator  = 20
	LevelViewer    = 10
)

// Role agrupa permisos. A diferencia del catalogo de permisos, los roles SI son
// datos: se crean, se editan y se borran desde el panel.
type Role struct {
	ID   int64
	Code string
	Name string
	// System marca los roles que crea el propio panel. No se pueden borrar,
	// para no dejar cuentas huerfanas ni el panel sin administrador.
	System bool
	// Level define la jerarquia. Ver LevelSuperuser y compania.
	Level       int
	Permissions PermissionSet
	CreatedAt   time.Time
}

// Outranks indica si este rol esta por encima del otro.
// La comparacion es estricta: un rol nunca supera a su igual.
func (r *Role) Outranks(other *Role) bool {
	if r == nil || other == nil {
		return false
	}
	return r.Level > other.Level
}

// IsSuperuser identifica el rol raiz: siempre tiene todos los permisos, no se
// puede editar, no se puede borrar y nadie puede gestionarlo.
func (r *Role) IsSuperuser() bool {
	return r != nil && r.Code == RoleCodeSuperuser
}

func (r *Role) Can(p Permission) bool {
	if r == nil {
		return false
	}
	return r.Permissions.Has(p)
}



// RootRoleDef define el unico rol que el sistema garantiza.
//
// Los demas roles NO se crean solos: son datos que se gestionan desde el panel.
// Imponerlos al arrancar contradiria la idea de que los roles son editables, y
// obligaria a borrar a mano lo que el arranque acaba de crear.
type RootRoleDef struct {
	Code  string
	Name  string
	Level int
}

// RootRole es el superusuario. Se asegura en cada arranque con el catalogo
// completo de permisos, y es lo unico que el panel impone.
var RootRole = RootRoleDef{
	Code:  RoleCodeSuperuser,
	Name:  "Superusuario",
	Level: LevelSuperuser,
}

// Niveles sugeridos para los roles que se creen desde el panel. No se aplican
// solos: son solo una referencia para elegir un numero coherente.
var SuggestedLevels = []struct {
	Name  string
	Level int
}{
	{"Administrador", LevelAdmin},
	{"Operador", LevelOperator},
	{"Espectador", LevelViewer},
}
