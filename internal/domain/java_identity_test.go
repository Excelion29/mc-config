package domain

import "testing"

// TestOfflineUUIDCoincideConMinecraft compara con valores calculados aparte.
//
// Los tres se sacaron con una implementacion independiente del mismo
// algoritmo -MD5 de "OfflinePlayer:<nombre>" con los bits de version 3 y
// variante RFC 4122-, no de ejecutar este codigo. Comparar una funcion consigo
// misma no comprueba nada.
//
// Vale la pena fijarlos porque un fallo aqui es invisible: el UUID tendria la
// pinta correcta, se escribiria en whitelist.json sin quejarse, y el servidor
// simplemente no reconoceria a nadie.
func TestOfflineUUIDCoincideConMinecraft(t *testing.T) {
	casos := map[string]string{
		"dante":          "56071dbf-3584-3531-8f51-d06000e6fc0d",
		"AmigoNoPremium": "c82af407-71a5-3888-bffd-8ea667d70661",
		"Notch":          "b50ad385-829d-3141-a216-7e7d7539ba7f",
	}

	for nombre, esperado := range casos {
		if got := OfflineUUID(nombre); got != esperado {
			t.Errorf("OfflineUUID(%q) = %q, se esperaba %q", nombre, got, esperado)
		}
	}
}

// TestElNombreDecideLaIdentidadSinConexion documenta por que hace falta AuthMe.
//
// El UUID sale del nombre y de nada mas: no hay nada que consultar ni nada que
// verificar. Quien escriba ese nombre al conectarse ES esa persona para el
// servidor. Esa es exactamente la razon de D-07.
func TestElNombreDecideLaIdentidadSinConexion(t *testing.T) {
	if OfflineUUID("dante") != OfflineUUID("dante") {
		t.Error("el mismo nombre deberia dar siempre el mismo UUID")
	}
	if OfflineUUID("dante") == OfflineUUID("Dante") {
		t.Error("Minecraft distingue mayusculas aqui; dos nombres distintos no pueden compartir UUID")
	}
}

// TestIdentidadesJavaSegunElModo protege el fallo que dejo fuera a un amigo.
//
// Un jugador sin cuenta comprada no tiene UUID de Mojang, asi que en modo
// normal no puede entrar y punto. Pero con el acceso abierto SI puede, y antes
// se le descartaba de la lista igualmente: el servidor le rechazaba sin
// mencionar la lista por ningun lado.
func TestIdentidadesJavaSegunElModo(t *testing.T) {
	noPremium := &Player{JavaName: "dante"}
	premium := &Player{JavaName: "wronkow", JavaUUID: "11111111-2222-3333-4444-555555555555"}

	if ids := noPremium.IdentidadesJava(AuthOnline); len(ids) != 0 {
		t.Errorf("sin cuenta comprada no deberia entrar en modo normal, dio %v", ids)
	}

	ids := noPremium.IdentidadesJava(AuthOffline)
	if len(ids) != 1 || ids[0] != OfflineUUID("dante") {
		t.Errorf("con el acceso abierto deberia valer el UUID calculado, dio %v", ids)
	}

	if ids := premium.IdentidadesJava(AuthOnline); len(ids) != 1 || ids[0] != premium.JavaUUID {
		t.Errorf("en modo normal solo vale el UUID de Mojang, dio %v", ids)
	}

	// Con el acceso abierto, quien SI compro el juego puede llegar con
	// cualquiera de los dos: lo decide un ajuste de FastLogin que no
	// escribimos. Van los dos para no dejarle fuera por adivinar mal.
	if ids := premium.IdentidadesJava(AuthOffline); len(ids) != 2 {
		t.Errorf("con el acceso abierto deberian valer los dos UUID, dio %v", ids)
	}

	if (&Player{}).PuedeJugarJavaEn(AuthOffline) {
		t.Error("sin nombre de Java no se puede jugar en ningun modo")
	}
}
