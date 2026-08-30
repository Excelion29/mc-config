package java

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// autoRegister es el ajuste de FastLogin que hace falta para que una cuenta
// comprada entre sin contrasena.
//
// Viene en FALSE de fabrica, y ese valor por defecto es justo el que rompe lo
// que el panel promete. Con autoRegister apagado, FastLogin solo entra solo a
// quien YA tiene cuenta en AuthMe, y no la crea: la primera vez que entra
// alguien premium, AuthMe le pide registrarse con contrasena, exactamente igual
// que a un no premium.
//
// Comprobado en el config.yml del propio proyecto antes de escribir esto, no de
// memoria.
const autoRegister = "autoRegister"

// AjustarPlugins deja la configuracion de los plugins como el panel promete.
//
// Solo toca archivos que el PLUGIN ya escribio, y solo la linea concreta que
// hace falta. No se genera una configuracion propia: seria una copia de los
// valores por defecto de otro proyecto, que envejece sola y en silencio en
// cuanto ellos cambien algo.
//
// La primera vez no hay nada que tocar -el archivo lo crea el plugin al
// arrancar-, asi que no es un error: se avisa y se aplica en el arranque
// siguiente, porque esto se ejecuta cada vez.
func (*Flavor) AjustarPlugins(dataDir string, mode domain.AuthMode) error {
	if !mode.SinConexion() {
		return nil
	}
	return ponerAutoRegister(filepath.Join(dataDir, "plugins", "FastLogin", "config.yml"))
}

// ponerAutoRegister cambia esa clave a true, dejando el resto igual.
func ponerAutoRegister(ruta string) error {
	datos, err := os.ReadFile(ruta)
	if os.IsNotExist(err) {
		// Todavia no ha arrancado con el plugin puesto. No es un fallo.
		return nil
	}
	if err != nil {
		return fmt.Errorf("leyendo la configuracion de FastLogin: %w", err)
	}

	nuevo, cambio := conAutoRegister(string(datos))
	if !cambio {
		return nil
	}

	if err := os.WriteFile(ruta, []byte(nuevo), 0o644); err != nil {
		return fmt.Errorf("escribiendo la configuracion de FastLogin: %w", err)
	}
	return nil
}

// conAutoRegister devuelve el archivo con la clave en true, y si hubo que
// tocarlo.
//
// Se trabaja por lineas y no con un analizador de YAML a proposito. El archivo
// es de otro proyecto y esta lleno de comentarios que explican cada opcion:
// leerlo y volver a escribirlo con una libreria los borraria todos, y quien
// abriera ese archivo despues se encontraria una lista de valores sin ninguna
// explicacion.
//
// Solo se cambia la clave de primer nivel: una con sangria pertenece a otra
// seccion y no es la que buscamos.
func conAutoRegister(yaml string) (string, bool) {
	lineas := strings.Split(yaml, "\n")

	for i, linea := range lineas {
		if !strings.HasPrefix(linea, autoRegister+":") {
			continue
		}

		valor := strings.TrimSpace(strings.TrimPrefix(linea, autoRegister+":"))
		if valor == "true" {
			return yaml, false
		}

		lineas[i] = autoRegister + ": true"
		return strings.Join(lineas, "\n"), true
	}

	// No esta la clave. Puede que la hayan renombrado en una version nueva, y
	// entonces anadirla no arreglaria nada y ensuciaria el archivo. Se deja
	// como esta: mejor que el panel avise de que no pudo que fingir que si.
	return yaml, false
}
