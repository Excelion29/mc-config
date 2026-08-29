package domain

import (
	"strings"
	"time"
)

// Pack es un paquete de texturas de la biblioteca.
//
// Es una cosa por derecho propio y no un campo del mundo: el mismo paquete
// suele valer para varios mapas, y un mapa puede traer varios. Guardarlo dentro
// del mundo obligaria a repetir el enlace en cada uno, y a corregirlos de uno
// en uno el dia que el autor lo mueva.
//
// Del archivo NO se guarda nada. Java sirve los paquetes por URL -el cliente la
// descarga solo al conectarse-, asi que alojarlo seria pagar almacenamiento y
// crecimiento de disco (M-2) por algo que Minecraft ya sabe hacer.
type Pack struct {
	ID   int64
	Name string
	// URL es de donde se descarga. Puede apuntar al archivo o a una pagina, y
	// eso decide si el servidor puede aplicarlo solo o hay que instalarlo a
	// mano.
	URL string
	// SHA1 es el hash que Java usa para NO volver a descargarlo en cada
	// conexion. Sin el, el cliente se lo baja entero cada vez que entra.
	//
	// Va vacio si no se pudo calcular, y entonces el paquete sigue
	// funcionando: solo se descarga de mas.
	SHA1 string
	// Note es para quien lo mira: de donde salio, de quien es, para que mapa.
	Note      string
	CreatedBy int64
	CreatedAt time.Time
}

// Automatico dice si el servidor puede aplicarlo sin que nadie haga nada.
//
// Solo cuando el enlace apunta al ARCHIVO. La mayoria de sitios de mapas dan
// una pagina de descarga -MediaFire, Drive- y el cliente no sabe que hacer con
// eso: se quedaria esperando un .zip y recibiria HTML.
//
// Se mira la extension y no se consulta la red: aqui no se puede bloquear
// pintando una pantalla, y una comprobacion que a veces tarda cinco segundos
// es peor que una regla que se explica en una linea.
func (p *Pack) Automatico() bool {
	if p == nil {
		return false
	}
	u := strings.ToLower(p.URL)
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	return strings.HasSuffix(u, ".zip")
}

// PackURLValida comprueba que el enlace se pueda usar.
//
// Solo https, y por un motivo distinto al de las portadas: aqui no lo descarga
// el navegador sino Minecraft, asi que no hay contenido mixto que valga. Lo que
// hay es que ese archivo acaba dentro del juego de cada persona que entra, y
// por http cualquiera en el camino puede cambiarlo sin que se note.
func PackURLValida(u string) bool {
	u = strings.TrimSpace(u)
	return u != "" && strings.HasPrefix(u, "https://")
}

// PackRef es lo que necesita un servidor para aplicar un paquete.
//
// Viaja hasta el adaptador de la edicion en la instancia, junto con las reglas,
// y se relee en cada arranque como ellas.
type PackRef struct {
	URL  string
	SHA1 string
	// Required echa a quien lo rechace. Lo decide cada mundo: un mapa de
	// aventura no se entiende sin su paquete, pero exigirlo en un mundo normal
	// deja fuera a quien tenga mala conexion.
	Required bool
}

// PackAsignado es un paquete con su papel dentro de un mundo.
type PackAsignado struct {
	Pack
	// Activo marca el unico que el servidor aplica solo.
	//
	// Es uno y no varios porque server.properties tiene UNA clave
	// "resource-pack". Apilar varios existe en las versiones modernas, pero
	// por API de plugin, y eso seria otro plugin que instalar y mantener.
	Activo bool
}
