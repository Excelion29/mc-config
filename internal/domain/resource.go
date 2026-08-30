package domain

import (
	"net/url"
	"path"
	"strings"
	"time"
)

// ResourceKind distingue los tipos de recurso.
//
// Existe desde el primer dia aunque solo haya uno: los que vengan -mods, mapas
// de referencia- tienen que caber como una fila mas, no como una tabla mas.
type ResourceKind string

const (
	// KindTexturePack cambia el aspecto del juego. Java lo sirve por enlace y
	// lo descarga el cliente al conectarse.
	KindTexturePack ResourceKind = "texture_pack"
)

func (k ResourceKind) Valid() bool { return k == KindTexturePack }

func (k ResourceKind) Label() string {
	switch k {
	case KindTexturePack:
		return "Paquete de texturas"
	}
	return string(k)
}

// TocaElMundo dice si el recurso puede cambiar los datos del mundo.
//
// Es la diferencia que de verdad importa entre un tipo y otro, y por eso vive
// en el tipo y no en un comentario:
//
//   - Un paquete de texturas NO lo toca. Es una capa por encima: cambia como se
//     ve, no lo que hay. Quitarlo no deja rastro y el mundo sigue igual.
//   - Un mod SI. Puede meter bloques y criaturas que solo el entiende, y
//     quitarlo despues deja el mundo con cosas que el juego no sabe leer.
//
// Se declara desde ya, con un solo tipo, porque el dia que entre el segundo la
// pantalla tiene que poder avisar sin que nadie se acuerde de anadir el aviso.
func (k ResourceKind) TocaElMundo() bool {
	switch k {
	case KindTexturePack:
		return false
	}
	// Ante un tipo que no conocemos se asume lo peor. Un aviso de mas se lee y
	// se ignora; uno de menos se descubre con el mundo ya roto.
	return true
}

// Impacto explica en una linea que le hace al mundo. Se ensena en pantalla.
func (k ResourceKind) Impacto() string {
	switch k {
	case KindTexturePack:
		return "Solo cambia como se ve. No toca el mundo: si lo quitas, todo sigue igual."
	}
	return "Puede cambiar los datos del mundo. Quitarlo despues no siempre se deshace."
}

// MaxRecursosPorMundo es cuantos puede llevar un mundo de cada tipo.
//
// El limite NO es por espacio: el panel no aloja nada, solo el enlace, que son
// unos cientos de bytes. Es por lo que se le pide a quien juega. Cada recurso
// que no se aplica solo es un enlace que alguien tiene que abrir, descargar e
// instalar antes de poder entrar, y a la tercera ya nadie lo hace.
const MaxRecursosPorMundo = 3

// Resource es algo de fuera que un mundo necesita: hoy, un paquete de texturas.
//
// Del archivo NO se guarda nada, solo el enlace. Es lo que Java hace de forma
// nativa -"resource-pack" en server.properties YA es una URL- asi que alojarlo
// seria pagar almacenamiento y crecimiento de disco (M-2) por algo que
// Minecraft ya sabe hacer.
//
// Se acepta a cambio que el archivo es de otro: si lo mueve o lo borra, el
// recurso deja de funcionar y desde aqui no se puede arreglar.
type Resource struct {
	ID   int64
	Kind ResourceKind

	// URL es lo UNICO obligatorio. Es lo que identifica al recurso de verdad.
	URL string

	// Name es opcional, y es una mascara del enlace: si esta, se ensena en vez
	// de la URL y al pulsarlo se va al enlace. Vacio es normal, porque casi
	// nadie tiene ganas de bautizar un enlace que acaba de pegar.
	Name string
	// AutoName es el titulo que el panel saco de la propia pagina al anadirlo.
	//
	// Se guarda aparte del nombre puesto a mano para no confundir lo que
	// alguien decidio con lo que el panel adivino: si el titulo de la pagina
	// era basura, se ve que no lo escribio nadie.
	AutoName string

	// SHA1 es la huella con la que el cliente reconoce el archivo que ya tiene.
	// Vacia si no se pudo calcular, y entonces solo se descarga de mas.
	SHA1 string

	// Directo dice si el enlace devolvio el ARCHIVO al abrirlo, y Probado si se
	// llego a comprobar.
	//
	// Son dos y no uno porque "es una pagina" y "no se pudo mirar" no son lo
	// mismo: con un solo campo, un fallo de red dejaria el recurso marcado como
	// manual para siempre.
	Directo bool
	Probado bool
	Note string

	CreatedBy int64
	CreatedAt time.Time
}

// Etiqueta es como se llama al recurso en pantalla.
//
// Manda el nombre puesto a mano; si no hay, el titulo que se saco de la pagina;
// y si tampoco, el propio enlace. Nunca queda en blanco: una fila sin nada que
// leer no se puede ni elegir ni borrar con criterio.
func (r *Resource) Etiqueta() string {
	if r == nil {
		return ""
	}
	if n := strings.TrimSpace(r.Name); n != "" {
		return n
	}
	if a := strings.TrimSpace(r.AutoName); a != "" {
		return a
	}
	return r.URL
}

// Bautizado dice si alguien le puso nombre, o si lo que se ensena es adivinado.
func (r *Resource) Bautizado() bool {
	return r != nil && strings.TrimSpace(r.Name) != ""
}

// Automatico dice si el servidor puede aplicarlo sin que el jugador haga nada.
//
// Solo cuando el enlace devuelve el ARCHIVO. Si devuelve una pagina de descarga
// -MediaFire, Drive- el cliente pide el paquete y recibe HTML: no carga las
// texturas y no dice por que.
//
// Manda lo que se vio al abrirlo, no como se llama la URL. Adivinar por la
// extension se equivocaba en los dos sentidos: hay CDN que sirven el archivo
// desde "/pack?id=123", y paginas que acaban en ".zip" sin serlo.
//
// Si no se pudo comprobar se cae en esa adivinanza, que es mejor que nada:
// bloquear un enlace correcto porque un servidor no respondio en su momento
// seria peor que arriesgarse a marcarlo mal.
func (r *Resource) Automatico() bool {
	if r == nil {
		return false
	}
	if r.Probado {
		return r.Directo
	}
	return ParecerDeArchivo(r.URL)
}

// ParecerDeArchivo es la adivinanza por el nombre, para cuando no se pudo mirar.
func ParecerDeArchivo(u string) bool {
	return strings.HasSuffix(strings.ToLower(rutaDe(u)), ".zip")
}

// TituloDeEnlace saca un nombre presentable de la propia URL.
//
// Es el ultimo recurso, para cuando no hay nombre puesto a mano ni titulo de la
// pagina. Solo vale la pena si de verdad dice algo:
//
//   - ".../texturas-medievales.zip"  ->  "texturas-medievales.zip"
//   - ".../download"                 ->  "ejemplo.com"
//
// Lo segundo paso de verdad y quedaba ridiculo: un recurso llamado "download".
// La ultima parte de una ruta no es un nombre por el hecho de estar al final;
// media internet acaba en /download, /file o /get, y esas palabras no
// distinguen un paquete de otro. Cuando no hay nada mejor, el dominio al menos
// dice de donde salio.
func TituloDeEnlace(u string) string {
	base := path.Base(rutaDe(u))

	if esNombreDeArchivo(base) {
		return base
	}

	parsed, err := url.Parse(strings.TrimSpace(u))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Host, "www.")
}

// esNombreDeArchivo distingue un archivo de un trozo de ruta cualquiera.
//
// Se exige extension: sin ella no hay forma de saber si "descargas" es un
// archivo o una carpeta. Y se descartan las palabras que usa todo el mundo para
// decir "aqui se baja", que estan al final de la ruta pero no nombran nada.
func esNombreDeArchivo(base string) bool {
	if base == "" || base == "." || base == "/" {
		return false
	}

	ext := path.Ext(base)
	if ext == "" || ext == base {
		return false
	}

	switch strings.ToLower(strings.TrimSuffix(base, ext)) {
	case "download", "descargar", "file", "archivo", "get", "dl", "index", "default":
		return false
	}
	return true
}

// rutaDe se queda con la ruta del enlace, sin parametros ni ancla.
//
// Hace falta porque un enlace puede acabar en ".zip?v=2" y seguir siendo un
// archivo, y porque un "?d=texturas.zip" NO lo es.
func rutaDe(u string) string {
	parsed, err := url.Parse(strings.TrimSpace(u))
	if err != nil {
		return ""
	}
	return parsed.Path
}

// RecursoURLValida comprueba que el enlace se pueda usar.
//
// Solo https, y por un motivo distinto al de las portadas: aquellas las carga el
// navegador y el problema era el contenido mixto. Este archivo lo descarga
// Minecraft y acaba dentro del juego de cada persona que entra, asi que por http
// cualquiera en el camino puede cambiarlo sin que se note.
func RecursoURLValida(u string) bool {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "https://") {
		return false
	}
	parsed, err := url.Parse(u)
	return err == nil && parsed.Host != ""
}

// PackRef es lo que necesita un servidor para aplicar el recurso principal.
//
// Se llama asi y no ResourceRef porque es exactamente lo que Minecraft llama
// "resource pack": viaja hasta server.properties.
type PackRef struct {
	URL  string
	SHA1 string
	// Required echa a quien lo rechace. Lo decide cada mundo: un mapa de
	// aventura no se entiende sin sus texturas, pero exigirlas en un mundo
	// normal deja fuera a quien tenga mala conexion.
	Required bool
}

// ResourceAsignado es un recurso con su papel dentro de un mundo.
type ResourceAsignado struct {
	Resource
	// Principal marca el unico que el servidor aplica solo.
	//
	// Es uno y no varios porque server.properties tiene UNA clave
	// "resource-pack". Apilar varios existe en las versiones modernas, pero por
	// API de plugin, y eso seria otro plugin que instalar y mantener con su
	// version fijada, como AuthMe.
	Principal bool
}
