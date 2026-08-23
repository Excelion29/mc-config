package bedrock

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

// Ping consulta el estado del servidor con un "unconnected ping" de RakNet.
//
// Es el protocolo que usa el propio Minecraft para mostrar la lista de
// servidores, asi que no hace falta ni sesion ni cuenta: responde aunque la
// allow-list este activa.
//
// Se implementa a mano en vez de traer una libreria de RakNet: son dos
// paquetes, y el panel corre junto a la produccion de un cliente (D-09), donde
// cada dependencia menos es una preocupacion menos.
//
// Hace falta para poder avisar antes de cambiar de mapa: por D-02 solo hay un
// servidor encendido, asi que cambiar desconecta a quien este jugando (D-08).

// magic identifica los paquetes sin conexion de RakNet. Es una constante del
// protocolo, no un valor nuestro.
var magic = []byte{
	0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe,
	0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78,
}

const (
	idUnconnectedPing = 0x01
	idUnconnectedPong = 0x1c
)

// Info es lo que el servidor responde al ping.
type Info struct {
	MOTD     string
	Version  string
	Protocol int
	Online   int
	Max      int
	Gamemode string
}

func Ping(ctx context.Context, host string, port int, timeout time.Duration) (*Info, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, fmt.Errorf("conectando a %s: %w", addr, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)

	if _, err := conn.Write(buildPing()); err != nil {
		return nil, fmt.Errorf("enviando el ping: %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("sin respuesta de %s: %w", addr, err)
	}

	return parsePong(buf[:n])
}

func buildPing() []byte {
	p := make([]byte, 0, 33)
	p = append(p, idUnconnectedPing)

	// Marca de tiempo: el servidor la devuelve tal cual.
	p = binary.BigEndian.AppendUint64(p, uint64(time.Now().UnixMilli()))
	p = append(p, magic...)
	// GUID del cliente: cualquiera sirve, solo debe ser distinto en cada envio.
	p = binary.BigEndian.AppendUint64(p, rand.Uint64())
	return p
}

// parsePong lee la respuesta.
//
// Formato: 0x1c + 8 bytes de tiempo + 8 de GUID + 16 de magic + 2 de longitud
// + una cadena con campos separados por ";":
//
//	MCPE;MOTD;protocolo;version;online;max;idServidor;subMOTD;modo;...
func parsePong(p []byte) (*Info, error) {
	const header = 1 + 8 + 8 + 16 + 2

	if len(p) < header {
		return nil, fmt.Errorf("respuesta demasiado corta (%d bytes)", len(p))
	}
	if p[0] != idUnconnectedPong {
		return nil, fmt.Errorf("respuesta inesperada: tipo 0x%02x", p[0])
	}

	declared := int(binary.BigEndian.Uint16(p[33:35]))
	payload := p[header:]
	// La longitud declarada puede exceder lo recibido si el datagrama vino
	// truncado: se usa lo que realmente hay.
	if declared < len(payload) {
		payload = payload[:declared]
	}

	fields := strings.Split(string(payload), ";")
	if len(fields) < 6 {
		return nil, fmt.Errorf("respuesta con %d campos, se esperaban al menos 6", len(fields))
	}

	info := &Info{
		MOTD:    fields[1],
		Version: fields[3],
	}
	info.Protocol, _ = strconv.Atoi(fields[2])
	info.Online, _ = strconv.Atoi(fields[4])
	info.Max, _ = strconv.Atoi(fields[5])
	if len(fields) > 8 {
		info.Gamemode = fields[8]
	}
	return info, nil
}
