// Package arch no contiene codigo de la aplicacion: existe para alojar la
// prueba que vigila la arquitectura.
//
// La regla de dependencias -domain no sabe de nadie, app solo sabe de domain,
// los adaptadores no se conocen entre si- no la garantiza el compilador. Un
// import de mas compila igual de bien, y para cuando se nota ya hay diez.
//
// Escribirla como prueba la convierte en algo que falla solo. Es la diferencia
// entre una arquitectura descrita en un documento y una que de verdad esta
// puesta.
package arch
