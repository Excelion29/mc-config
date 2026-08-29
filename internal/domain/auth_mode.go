package domain

// AuthMode dice quien autentica a los jugadores.
//
// Es un ajuste GLOBAL del panel, no de cada mundo (D-17). La razon es la
// identidad: en modo sin conexion el UUID de un jugador se calcula a partir de
// su nombre, y en modo normal lo asigna Mojang. Si cada mundo pudiera elegir,
// la misma persona tendria dos UUID distintos segun donde jugara, y las listas
// de acceso tendrian que llevar los dos y acertar con cual usar.
type AuthMode string

const (
	// AuthOnline: autentica Mojang. Solo entran cuentas compradas, y nadie
	// puede usar el nombre de otro. Es el modo seguro y el de por defecto.
	AuthOnline AuthMode = "online"
	// AuthOffline: el servidor no comprueba nada con Mojang, asi que pueden
	// entrar cuentas no compradas.
	//
	// Por si solo es PELIGROSO: cualquiera puede escribir el nombre que quiera,
	// incluido el tuyo con tus permisos. Lo que lo hace usable son dos plugins
	// (D-07):
	//
	//	AuthMe     exige contrasena, asi que un nombre sin la contrasena no
	//	           sirve para entrar.
	//	FastLogin  reconoce las cuentas compradas de verdad, las autentica
	//	           contra Mojang sin contrasena, e IMPIDE que un no premium use
	//	           un nombre premium.
	//
	// Por eso el panel no permite activarlo si los plugins no estan puestos.
	AuthOffline AuthMode = "offline"
)

func (a AuthMode) Valid() bool { return a == AuthOnline || a == AuthOffline }

func (a AuthMode) Label() string {
	switch a {
	case AuthOnline:
		return "Solo cuentas compradas"
	case AuthOffline:
		return "Tambien cuentas no premium"
	}
	return string(a)
}

// SinConexion indica si el servidor debe arrancar sin comprobar con Mojang.
func (a AuthMode) SinConexion() bool { return a == AuthOffline }

// Claves de la tabla de ajustes.
const SettingAuthMode = "auth_mode"
