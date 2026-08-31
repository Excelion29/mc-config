package java

import (
	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// AuthMe es lo que hace usable el modo sin conexion, con la URL y el nombre de
// archivo VERIFICADOS a mano sobre Minecraft 26.2.
//
// Se apunta a la pagina de lanzamientos del proyecto y no a un repositorio de
// plugins: SpigotMC bloquea las descargas automaticas de AuthMe -devuelve un
// error que dice literalmente que lo bajes de GitHub- y con eso el arranque
// falla sin explicar nada.
//
// Y se fija la version en la URL a proposito. Un plugin de autenticacion que se
// actualiza solo es lo ultimo que queremos: si un dia hay que subirlo, se cambia
// aqui y el cambio queda escrito y visible en un diff.
const (
	authMeFile = "AuthMe-6.0.0-Paper.jar"
	authMeURL  = "https://github.com/AuthMe/AuthMeReloaded/releases/download/" +
		"6.0.0/AuthMe-6.0.0-Paper.jar"
)

// PluginsFor da los plugins que exige un modo de autenticacion.
//
// En modo normal, ninguno: Mojang autentica y no hace falta nada mas. AuthMe
// existe para arreglar lo que rompe el modo sin conexion, y por eso solo aparece
// ahi.
func (*Flavor) PluginsFor(mode domain.AuthMode) []app.Plugin {
	if !mode.SinConexion() {
		return nil
	}

	return []app.Plugin{
		{
			ID:   "authme",
			Name: "AuthMe",
			File: authMeFile,
			URL:  authMeURL,
			Docs: "https://github.com/AuthMe/AuthMeReloaded",
			Why: "Pide una contrasena al entrar. Sin esto, en modo sin " +
				"conexion cualquiera podria usar el nombre de otro.",
		},
	}
}

// FastLogin NO esta en la lista, y no es un olvido (2026-08-30).
//
// Haria que una cuenta comprada entrara sin contrasena, verificandola contra
// Mojang de verdad. Se instalo, y el servidor lo apago solo:
//
//	[FastLogin] Either ProtocolLib or ProtocolSupport have to be installed
//	            if you don't use BungeeCord
//	[FastLogin] Safely shutting down scheduler.
//
// En un servidor directo -sin proxy delante- FastLogin necesita ProtocolLib para
// interceptar los paquetes de login. Y ProtocolLib NO tiene version estable para
// 26.2: la ultima, 5.4.0, llega hasta 1.21.8. Solo existe una etiqueta rodante
// "dev-build" cuyo archivo cambia sin avisar.
//
// Poner esa etiqueta contradiria la regla de arriba justo en la pieza mas
// delicada: el plugin del que depende quien entra, actualizandose solo y sin que
// nadie lo note, porque el servidor arranca igual.
//
// Asi que se acepta lo que si funciona: TODO EL MUNDO usa contrasena, tambien
// quien compro el juego. Lo que se pierde es comodidad, no seguridad: la
// whitelist sigue decidiendo quien entra. Lo que queda expuesto es que, entre
// los invitados, alguien podria registrar el nombre de otro si llega primero.
//
// Volvera a la lista el dia que haya ProtocolLib estable para esta version, o si
// se monta un proxy -con proxy, FastLogin no lo necesita-.

// Variantes que NO se usan, anotadas para que nadie las elija por error:
//
//	AuthMe-6.0.0-Bungee.jar   para cuando hay un proxy BungeeCord delante
//	AuthMe-6.0.0-Folia.jar    para Folia, que no es Paper
//
// Aqui no hay proxy: el cliente habla directamente con Paper. Poner la variante
// de proxy da un fallo confuso, porque el plugin carga y luego no encuentra al
// intermediario que espera.
