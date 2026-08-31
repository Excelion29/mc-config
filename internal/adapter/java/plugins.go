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

	// SkinsRestorer devuelve las skins, que en modo sin conexion se pierden:
	// el servidor no le pregunta a Mojang la apariencia de nadie, y todos
	// salen con la piel por defecto.
	//
	// Se comprobo en Modrinth, que publica las versiones soportadas de cada
	// lanzamiento, que la 15.12.5 incluye 26.2 para el cargador de Paper. Y NO
	// depende de ProtocolLib, que es lo que dejo fuera a FastLogin.
	//
	// Se descarga de sus lanzamientos en GitHub y no de SpigotMC, por lo mismo
	// que AuthMe: alli las descargas automaticas se responden con un 403.
	skinsFile = "SkinsRestorer.jar"
	skinsURL  = "https://github.com/SkinsRestorer/SkinsRestorer/releases/download/" +
		"15.12.5/SkinsRestorer.jar"
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
			// Es LO que hace seguro el modo sin conexion: sin el, no se abre.
			Esencial: true,
		},
		{
			ID:   "skinsrestorer",
			Name: "SkinsRestorer",
			File: skinsFile,
			URL:  skinsURL,
			Docs: "https://skinsrestorer.net/docs",
			Why: "Devuelve las skins. Sin conexion el servidor no le pregunta " +
				"a Mojang que aspecto tiene cada uno, y todos entran con la " +
				"piel por defecto.",
			// NO es esencial: sin el se pierden las skins, no la puerta.
			// Negarse a abrir el acceso por esto seria confundir la comodidad
			// con la seguridad.
			Esencial: false,
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
