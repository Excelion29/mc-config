// Package app contiene los casos de uso y los puertos que necesitan.
//
// Los puertos se declaran aqui, donde se CONSUMEN, no en el paquete que los
// implementa. Es lo idiomatico en Go -las interfaces son implicitas, asi que
// quien las cumple no necesita importarlas- y es lo que mantiene la flecha de
// dependencia apuntando siempre hacia adentro:
//
//	adapter/web  ->  app  ->  domain
//	adapter/sqlite   (implementa los puertos de app sin conocerlos)
//
// Cada puerto vive junto al caso de uso que lo usa, en ports_<tema>.go, y no
// en un archivo unico: un cajon con todas las interfaces del proyecto obliga a
// leerlo entero para trabajar en una sola pantalla.
package app

import (
	"time"
)

// Clock permite fijar el tiempo en los tests. En produccion es time.Now.
type Clock func() time.Time
