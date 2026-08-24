package java

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

// TestVarInt fija la codificacion que hace especial a este protocolo: siete
// bits por byte, y el octavo dice "sigue". Los valores son los del ejemplo
// canonico del protocolo de Minecraft.
func TestVarInt(t *testing.T) {
	casos := []struct {
		valor  int
		bytes  []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7f}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xff, 0x01}},
		{25565, []byte{0xdd, 0xc7, 0x01}},
		{2097151, []byte{0xff, 0xff, 0x7f}},
		// -1 es el que mandamos en el handshake para decir "no se que version
		// soy". Ocupa los cinco bytes.
		{-1, []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
	}

	for _, c := range casos {
		got := appendVarInt(nil, c.valor)
		if !bytes.Equal(got, c.bytes) {
			t.Errorf("appendVarInt(%d) = % x, se esperaba % x", c.valor, got, c.bytes)
		}

		vuelta, err := leerVarInt(bufio.NewReader(bytes.NewReader(c.bytes)))
		if err != nil {
			t.Errorf("leerVarInt(% x): %v", c.bytes, err)
			continue
		}
		if vuelta != c.valor {
			t.Errorf("leerVarInt(% x) = %d, se esperaba %d", c.bytes, vuelta, c.valor)
		}
	}
}

// TestLeerVarIntRechazaBasura: sin tope, un byte con el bit de continuacion
// siempre puesto haria girar el bucle para siempre.
func TestLeerVarIntRechazaBasura(t *testing.T) {
	infinito := bytes.Repeat([]byte{0x80}, 20)
	if _, err := leerVarInt(bufio.NewReader(bytes.NewReader(infinito))); err == nil {
		t.Error("se esperaba un error con un entero variable interminable")
	}
}

// TestPlayersContraUnServidorFalso levanta un servidor que habla el protocolo
// de verdad y comprueba la conversacion entera.
//
// Es la unica forma de probar esto sin un Minecraft delante: el servidor falso
// LEE el handshake y valida que lo que mandamos es lo que toca, en vez de
// contestar a ciegas.
func TestPlayersContraUnServidorFalso(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	errc := make(chan error, 1)
	go func() { errc <- servidorFalso(ln) }()

	puerto := ln.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	online, max, err := (&Flavor{}).Players(ctx, "127.0.0.1", puerto)
	if err != nil {
		t.Fatalf("Players: %v", err)
	}
	if online != 3 || max != 12 {
		t.Errorf("Players = %d/%d, se esperaba 3/12", online, max)
	}
	if err := <-errc; err != nil {
		t.Errorf("el servidor falso se quejo: %v", err)
	}
}

// servidorFalso habla el lado del servidor y comprueba lo que recibe.
func servidorFalso(ln net.Listener) error {
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	r := bufio.NewReader(conn)

	// --- Handshake ---
	if _, err := leerVarInt(r); err != nil { // longitud
		return err
	}
	id, err := leerVarInt(r)
	if err != nil {
		return err
	}
	if id != 0x00 {
		return errf("handshake con identificador %d", id)
	}
	if _, err := leerVarInt(r); err != nil { // version del protocolo
		return err
	}
	if _, err := leerString(r); err != nil { // direccion
		return err
	}
	var puerto uint16
	if err := binary.Read(r, binary.BigEndian, &puerto); err != nil {
		return err
	}
	siguiente, err := leerVarInt(r)
	if err != nil {
		return err
	}
	if siguiente != 1 {
		return errf("se pidio el estado %d, deberia ser 1", siguiente)
	}

	// --- Status Request ---
	if _, err := leerVarInt(r); err != nil {
		return err
	}
	if id, err := leerVarInt(r); err != nil || id != 0x00 {
		return errf("peticion de estado invalida (id=%d, err=%v)", id, err)
	}

	// --- Status Response ---
	json := `{"version":{"name":"26.2"},"players":{"online":3,"max":12}}`
	var cuerpo []byte
	cuerpo = appendString(cuerpo, json)
	return enviarPaquete(conn, 0x00, cuerpo)
}

func errf(formato string, args ...any) error {
	return fmt.Errorf(formato, args...)
}
