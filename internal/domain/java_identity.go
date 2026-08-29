package domain

import (
	"crypto/md5"
	"fmt"
)

// OfflineUUID calcula el UUID que Minecraft asigna a un jugador SIN cuenta
// premium, a partir de su nombre.
//
// No hay nada que consultar: es determinista. Java toma el MD5 de la cadena
// "OfflinePlayer:<nombre>" y lo convierte en un UUID de version 3. Por eso un
// jugador no premium tiene siempre el mismo UUID en cualquier servidor sin
// conexion, y por eso su nombre ES su identidad -y por eso hace falta AuthMe
// para que nadie use el nombre de otro (D-07)-.
//
// Vive en el dominio y no en el adaptador de Java porque no es hablar con
// nadie: es la regla de como se identifica una persona sin cuenta comprada. La
// necesitan tanto el alta de jugadores como la escritura de la lista.
func OfflineUUID(nombre string) string {
	suma := md5.Sum([]byte("OfflinePlayer:" + nombre))

	// Se marcan version 3 y variante RFC 4122, que es lo que hace la funcion
	// equivalente de Java al construir el UUID desde unos bytes.
	b := suma
	b[6] = (b[6] & 0x0f) | 0x30
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// IdentidadesJava da los UUID con los que un jugador puede llegar a un servidor
// de Java, segun el modo vigente.
//
// Devuelve una lista y no un valor porque en modo sin conexion hay DOS posibles
// para quien si compro el juego, y cual llega no lo decidimos nosotros:
// FastLogin tiene un ajuste -premiumUuid- que elige entre darle su UUID de
// Mojang o el que le tocaria sin conexion. No escribimos ese archivo, asi que
// no podemos afirmar cual es.
//
// Ante esa duda se ponen los dos en la lista. Una entrada de mas no abre nada
// -sigue siendo esa persona con ese nombre- mientras que faltar la que sea deja
// fuera a alguien que si tiene permiso, y con un rechazo que no explica por que.
func (p *Player) IdentidadesJava(modo AuthMode) []string {
	if p == nil || p.JavaName == "" {
		return nil
	}

	if !modo.SinConexion() {
		// Con Mojang autenticando, el unico UUID valido es el suyo. El de sin
		// conexion aqui seria un desconocido.
		if p.JavaUUID == "" {
			return nil
		}
		return []string{p.JavaUUID}
	}

	ids := []string{OfflineUUID(p.JavaName)}
	if p.JavaUUID != "" {
		ids = append(ids, p.JavaUUID)
	}
	return ids
}

// PuedeJugarJavaEn dice si el jugador tiene identidad valida en un modo dado.
//
// En modo sin conexion basta el nombre: el UUID se calcula. Es la diferencia
// que hace posible dar de alta a alguien que no compro el juego, porque a ese
// Mojang no le va a saber decir nada.
func (p *Player) PuedeJugarJavaEn(modo AuthMode) bool {
	return len(p.IdentidadesJava(modo)) > 0
}
