package java

import (
	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// Los dos plugins que hacen usable el modo sin conexion, con las URL y los
// nombres de archivo VERIFICADOS a mano el 2026-08-24 sobre Minecraft 26.2.
//
// Se apunta a la pagina de lanzamientos de cada proyecto y no a un repositorio
// de plugins: SpigotMC bloquea las descargas automaticas de AuthMe -devuelve un
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

	// El proyecto cambio de dueno: antes era games647, ahora TuxCoding. La URL
	// vieja devuelve vacio sin dar error.
	fastLoginFile = "FastLoginBukkit.jar"
	fastLoginURL  = "https://github.com/TuxCoding/FastLogin/releases/download/" +
		"1.12-kick-toggle/FastLoginBukkit.jar"
)

// PluginsFor da los plugins que exige un modo de autenticacion.
//
// En modo normal, ninguno: Mojang autentica y no hace falta nada mas. Los dos
// de abajo existen para arreglar lo que rompe el modo sin conexion, y por eso
// solo aparecen ahi.
func (*Flavor) PluginsFor(mode domain.AuthMode) []app.Plugin {
	if !mode.SinConexion() {
		return nil
	}

	return []app.Plugin{
		{
			Name: "AuthMe",
			File: authMeFile,
			URL:  authMeURL,
			Why: "Pide una contrasena al entrar. Sin esto, en modo sin " +
				"conexion cualquiera podria usar el nombre de otro.",
		},
		{
			Name: "FastLogin",
			File: fastLoginFile,
			URL:  fastLoginURL,
			Why: "Reconoce las cuentas compradas y las deja entrar sin " +
				"contrasena, e impide que alguien sin comprar use un nombre " +
				"que si lo esta.",
		},
	}
}

// Variantes que NO se usan, anotadas para que nadie las elija por error:
//
//	AuthMe-6.0.0-Bungee.jar   para cuando hay un proxy BungeeCord delante
//	AuthMe-6.0.0-Folia.jar    para Folia, que no es Paper
//	FastLoginBungee.jar       idem, proxy
//	FastLoginVelocity.jar     idem, proxy Velocity
//
// Aqui no hay proxy: el cliente habla directamente con Paper. Poner la variante
// de proxy da un fallo confuso, porque el plugin carga y luego no encuentra al
// intermediario que espera.
