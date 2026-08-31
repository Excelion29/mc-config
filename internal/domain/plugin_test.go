package domain

import (
	"errors"
	"testing"
)

// TestElEnlaceTieneQueLlevarAUnJar protege un fallo que no da error.
//
// Lo que se ponga en plugins/ que no sea un .jar, el servidor lo IGNORA: arranca
// perfectamente, sin el complemento, y el panel se queda diciendo que esta
// instalado porque el archivo esta ahi.
//
// Por eso se comprueba al pegar el enlace, que es cuando todavia hay una
// pantalla donde decirlo.
func TestElEnlaceTieneQueLlevarAUnJar(t *testing.T) {
	buenos := map[string]string{
		"https://github.com/AuthMe/AuthMeReloaded/releases/download/6.0.0/AuthMe-6.0.0-Paper.jar": "AuthMe-6.0.0-Paper.jar",
		"https://ejemplo.com/plugins/Algo.JAR":                                                    "Algo.JAR",
		"https://ejemplo.com/x/Algo.jar?token=abc":                                                "Algo.jar",
	}
	for enlace, esperado := range buenos {
		got, err := ArchivoDeJar(enlace)
		if err != nil {
			t.Errorf("ArchivoDeJar(%q) fallo: %v", enlace, err)
			continue
		}
		if got != esperado {
			t.Errorf("ArchivoDeJar(%q) = %q, se esperaba %q", enlace, got, esperado)
		}
	}

	malos := []string{
		"",
		"   ",
		"http://ejemplo.com/x.jar",               // sin https
		"https://ejemplo.com/releases/tag/6.0.0", // la pagina, no el archivo
		"https://ejemplo.com/x.zip",              // otro tipo de archivo
		"https://ejemplo.com/",
		"https://ejemplo.com/.jar", // sin nombre
		"ejemplo.com/x.jar",
	}
	for _, enlace := range malos {
		if _, err := ArchivoDeJar(enlace); !errors.Is(err, ErrJarInvalido) {
			t.Errorf("ArchivoDeJar(%q) deberia rechazarse, dio %v", enlace, err)
		}
	}
}
