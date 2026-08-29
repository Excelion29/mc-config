package app

import (
	"context"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Plugin es un complemento que un servidor necesita para hacer algo que por su
// cuenta no puede.
//
// El panel NO ofrece "gestionar plugins": ofrece capacidades. Quien lo usa
// quiere que entren sus amigos sin Minecraft comprado, no administrar archivos
// .jar. Los plugins son el como, y por eso esta lista la decide el adaptador de
// cada edicion y no la persona.
type Plugin struct {
	// Name es el nombre para las personas, no el del archivo.
	Name string
	// File es como se llama el archivo dentro de plugins/. Importa porque es
	// lo que permite saber si ya esta instalado sin volver a descargarlo.
	File string
	// URL de donde se baja. Se apunta a la pagina de lanzamientos del propio
	// proyecto y no a un repositorio de plugins: SpigotMC bloquea las descargas
	// automaticas de algunos, y el fallo llega como un 403 sin explicacion.
	URL string
	// Why explica, en una frase, para que hace falta. Se ensena en el panel:
	// quien administra tiene derecho a saber que se le esta instalando.
	Why string
}

// PluginStore descarga los complementos y los deja en la instancia.
//
// Es un puerto porque bajar archivos y copiarlos es infraestructura: los casos
// de uso solo saben que un servidor "necesita estos plugins".
type PluginStore interface {
	// Install deja los plugins en dataDir, descargando los que falten.
	//
	// La descarga se cachea fuera de la instancia: los mismos plugins valen
	// para todos los servidores, y bajarlos una vez por servidor seria gastar
	// red y disco para tener copias identicas.
	Install(ctx context.Context, dataDir string, plugins []Plugin) error
	// Installed dice que plugins de la lista ya estan en dataDir.
	//
	// Hace falta para poder NEGARSE a activar el modo sin conexion cuando los
	// plugins no estan: sin ellos, ese modo deja el servidor abierto a que
	// cualquiera entre con el nombre que quiera.
	Installed(dataDir string, plugins []Plugin) []Plugin
}

// PluginProvider lo implementa cada edicion para decir que necesita.
//
// Vive aqui y no en ServerFlavor porque depende del MODO de autenticacion, que
// es un ajuste del panel y no de la edicion.
type PluginProvider interface {
	// PluginsFor da los plugins que exige ese modo. En modo normal, ninguno.
	PluginsFor(mode domain.AuthMode) []Plugin
}
