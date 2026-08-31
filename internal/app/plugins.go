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
	// ID identifica al complemento a lo largo del tiempo.
	//
	// Es estable a proposito: el nombre visible y el archivo cambian al subir
	// de version, asi que ninguno de los dos sirve para recordar "de este ya
	// elegiste otra version".
	ID string
	// Name es el nombre para las personas, no el del archivo.
	Name string
	// File es como se llama el archivo dentro de plugins/. Importa porque es
	// lo que permite saber si ya esta instalado sin volver a descargarlo.
	File string
	// URL de donde se baja. Se apunta a la pagina de lanzamientos del propio
	// proyecto y no a un repositorio de plugins: SpigotMC bloquea las descargas
	// automaticas de algunos, y el fallo llega como un 403 sin explicacion.
	URL string
	// Docs es la pagina del proyecto. Se ensena como enlace en el panel.
	//
	// Esta porque quien tiene que decidir si sube de version necesita poder
	// leer que cambia, y buscarlo por su cuenta es pedirle que confie a ciegas
	// en un .jar que el panel le instala.
	Docs string
	// Why explica, en una frase, para que hace falta. Se ensena en el panel:
	// quien administra tiene derecho a saber que se le esta instalando.
	Why string
	// DeFabrica dice si lleva la version que trae el codigo, o una que alguien
	// eligio desde el panel.
	DeFabrica bool
}

// PluginVersion es la version que alguien eligio para un complemento.
type PluginVersion struct {
	PluginID string
	URL      string
	File     string
}

// PluginVersionRepo guarda las versiones elegidas desde el panel.
//
// Solo guarda lo que se cambio: sin fila, manda lo que dice el codigo. Asi un
// panel recien instalado arranca con lo verificado a mano, y cada fila de esta
// tabla es una decision explicita de alguien.
type PluginVersionRepo interface {
	All(ctx context.Context) (map[string]PluginVersion, error)
	Set(ctx context.Context, v PluginVersion, by int64) error
	Clear(ctx context.Context, pluginID string) error
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
	// Remove borra un .jar de la instancia.
	//
	// Hace falta al cambiar de version: dos .jar del mismo plugin en la misma
	// carpeta se cargan LOS DOS, y el servidor se pelea consigo mismo.
	Remove(dataDir, file string) error
}

// PluginProvider lo implementa cada edicion para decir que necesita.
//
// Vive aqui y no en ServerFlavor porque depende del MODO de autenticacion, que
// es un ajuste del panel y no de la edicion.
type PluginProvider interface {
	// PluginsFor da los plugins que exige ese modo. En modo normal, ninguno.
	PluginsFor(mode domain.AuthMode) []Plugin
}

