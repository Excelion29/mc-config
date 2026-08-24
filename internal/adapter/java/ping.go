package java

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// Players consulta cuanta gente hay conectada con el "Server List Ping" de
// Java, el mismo que usa el juego para pintar tu lista de servidores.
//
// No tiene NADA que ver con el ping de Bedrock. Aquel eran dos datagramas UDP
// sueltos; este es una conversacion TCP con paquetes con longitud, enteros de
// tamano variable y una respuesta en JSON.
//
// La conversacion es:
//
//	-> Handshake      version del protocolo, direccion, puerto, "quiero estado"
//	-> Status Request vacio
//	<- Status Response JSON con {"players":{"online":N,"max":M}, ...}
//
// Se implementa a mano, como el de Bedrock: son ochenta lineas y evita traer
// una biblioteca entera para leer dos numeros.
func (*Flavor) Players(ctx context.Context, host string, port int) (int, int, error) {
	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 0, 0, fmt.Errorf("conectando al servidor: %w", err)
	}
	defer conn.Close()

	// Si el servidor esta arrancando puede aceptar la conexion y no contestar.
	// Sin plazo, la consulta se quedaria colgada y con ella la pagina.
	plazo, _ := ctx.Deadline()
	if plazo.IsZero() {
		plazo = time.Now().Add(3 * time.Second)
	}
	if err := conn.SetDeadline(plazo); err != nil {
		return 0, 0, err
	}

	if err := enviarHandshake(conn, host, port); err != nil {
		return 0, 0, err
	}
	// Status Request: un paquete vacio con identificador 0.
	if err := enviarPaquete(conn, 0x00, nil); err != nil {
		return 0, 0, err
	}

	estado, err := leerEstado(bufio.NewReader(conn))
	if err != nil {
		return 0, 0, err
	}
	return estado.Players.Online, estado.Players.Max, nil
}

// versionProtocoloConsulta es la version que se declara en el handshake.
//
// -1 significa "no lo se todavia", y es lo que usan las herramientas de
// consulta. El servidor responde igual con su estado, y asi no hay que ir
// actualizando un numero cada vez que sale una version de Minecraft. Pedirle
// una version concreta seria inventarse una compatibilidad que no hace falta:
// solo queremos leer cuanta gente hay.
const versionProtocoloConsulta = -1

func enviarHandshake(w io.Writer, host string, port int) error {
	var cuerpo []byte
	cuerpo = appendVarInt(cuerpo, versionProtocoloConsulta)
	cuerpo = appendString(cuerpo, host)
	cuerpo = binary.BigEndian.AppendUint16(cuerpo, uint16(port))
	cuerpo = appendVarInt(cuerpo, 1) // 1 = quiero el estado, no entrar a jugar

	return enviarPaquete(w, 0x00, cuerpo)
}

// enviarPaquete escribe un paquete: longitud, identificador y cuerpo.
func enviarPaquete(w io.Writer, id int, cuerpo []byte) error {
	var contenido []byte
	contenido = appendVarInt(contenido, id)
	contenido = append(contenido, cuerpo...)

	var paquete []byte
	paquete = appendVarInt(paquete, len(contenido))
	paquete = append(paquete, contenido...)

	_, err := w.Write(paquete)
	return err
}

type estadoServidor struct {
	Players struct {
		Online int `json:"online"`
		Max    int `json:"max"`
	} `json:"players"`
	Version struct {
		Name string `json:"name"`
	} `json:"version"`
}

func leerEstado(r *bufio.Reader) (*estadoServidor, error) {
	longitud, err := leerVarInt(r)
	if err != nil {
		return nil, fmt.Errorf("leyendo la longitud de la respuesta: %w", err)
	}
	// Un servidor con muchos jugadores manda una lista de nombres, pero
	// megabytes ya no son un estado: seria alguien mandando basura.
	if longitud <= 0 || longitud > 1<<20 {
		return nil, fmt.Errorf("respuesta de tamano imposible: %d", longitud)
	}

	id, err := leerVarInt(r)
	if err != nil {
		return nil, err
	}
	if id != 0x00 {
		return nil, fmt.Errorf("se esperaba un paquete de estado, llego el %d", id)
	}

	texto, err := leerString(r)
	if err != nil {
		return nil, fmt.Errorf("leyendo el estado: %w", err)
	}

	var estado estadoServidor
	if err := json.Unmarshal([]byte(texto), &estado); err != nil {
		return nil, fmt.Errorf("el estado no es JSON valido: %w", err)
	}
	return &estado, nil
}

// --- Enteros de tamano variable -------------------------------------------
//
// Minecraft codifica los enteros en siete bits por byte, usando el octavo para
// decir "sigue". Un numero pequeno ocupa un byte y uno grande hasta cinco. Es
// lo que hace que no se pueda leer el protocolo con encoding/binary a secas.

func appendVarInt(b []byte, v int) []byte {
	u := uint32(v)
	for {
		if u&^0x7F == 0 {
			return append(b, byte(u))
		}
		b = append(b, byte(u&0x7F|0x80))
		u >>= 7
	}
}

func leerVarInt(r io.ByteReader) (int, error) {
	var valor uint32
	for i := 0; i < 5; i++ { // 5 bytes es el maximo de un entero de 32 bits
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		valor |= uint32(b&0x7F) << (7 * i)
		if b&0x80 == 0 {
			return int(int32(valor)), nil
		}
	}
	return 0, fmt.Errorf("entero variable demasiado largo")
}

func appendString(b []byte, s string) []byte {
	b = appendVarInt(b, len(s))
	return append(b, s...)
}

func leerString(r *bufio.Reader) (string, error) {
	n, err := leerVarInt(r)
	if err != nil {
		return "", err
	}
	if n < 0 || n > 1<<20 {
		return "", fmt.Errorf("cadena de tamano imposible: %d", n)
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}
