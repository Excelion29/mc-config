package domain

import (
	"net/url"
	"path"
	"strings"
)

// ArchivoDeJar saca el nombre del archivo del enlace de un complemento.
//
// Se deduce del enlace y no se pide aparte porque quien sube de version copia
// una URL, no rellena un formulario: pedirle ademas el nombre exacto del archivo
// seria pedirle que copie dos veces lo mismo y acertar en las dos.
//
// Se exige que acabe en .jar, y no por gusto. Un archivo que no lo sea en la
// carpeta plugins/ NO da error: el servidor simplemente lo ignora, arranca sin
// el complemento, y el panel se queda diciendo que esta instalado.
func ArchivoDeJar(enlace string) (string, error) {
	enlace = strings.TrimSpace(enlace)
	if !strings.HasPrefix(enlace, "https://") {
		return "", ErrJarInvalido
	}

	parsed, err := url.Parse(enlace)
	if err != nil || parsed.Host == "" {
		return "", ErrJarInvalido
	}

	nombre := path.Base(parsed.Path)
	if !strings.HasSuffix(strings.ToLower(nombre), ".jar") {
		return "", ErrJarInvalido
	}

	// Un nombre con separadores dentro escribiria fuera de plugins/. Base ya lo
	// evita, pero la comprobacion se queda escrita: es la clase de detalle que
	// alguien "simplifica" sin ver para que estaba.
	if strings.ContainsAny(nombre, `/\`) || nombre == ".jar" {
		return "", ErrJarInvalido
	}
	return nombre, nil
}
