package app

import "github.com/Excelion29/mc-config/internal/domain"

// Umbrales de ocupacion del disco (M-2).
//
// Dos escalones y no uno, porque avisar y bloquear responden a preguntas
// distintas: "esto va a acabar mal" y "esto ya no cabe".
const (
	// DiscoAviso: a partir de aqui el panel lo dice en pantalla, pero deja
	// trabajar. Es el 80% de M-2.
	DiscoAviso = 80
	// DiscoTope: a partir de aqui no se aceptan mapas nuevos.
	//
	// El corte esta ANTES de llenarse a proposito. Esta VPS la comparte el
	// proyecto de un cliente, y un disco al 100% no rompe solo a MCVPS: MySQL
	// deja de poder escribir y se cae lo del cliente tambien. El margen que
	// queda es el que le permite seguir funcionando mientras se hace sitio.
	DiscoTope = 90
)

// DiskStatus es la foto del disco donde viven mapas y mundos.
type DiskStatus struct {
	Libre uint64
	Total uint64
	// Ocupado es el porcentaje usado, redondeado hacia arriba: mas vale que
	// diga 81 cuando va por 80.4 que al reves.
	Ocupado int
}

// Avisar indica si conviene ensenarlo en pantalla.
func (d DiskStatus) Avisar() bool { return d.Ocupado >= DiscoAviso }

// Lleno indica si hay que dejar de aceptar mapas.
func (d DiskStatus) Lleno() bool { return d.Ocupado >= DiscoTope }

// LibreLegible da el espacio libre en unidades humanas.
func (d DiskStatus) LibreLegible() string { return HumanSize(int64(d.Libre)) }

// Disk consulta el estado del disco.
func (m *Maps) Disk() (DiskStatus, error) {
	libre, total, err := m.store.DiskUsage()
	if err != nil {
		return DiskStatus{}, err
	}
	if total == 0 {
		return DiskStatus{}, nil
	}

	usado := total - libre
	// Se redondea hacia arriba con aritmetica entera para no arrastrar coma
	// flotante en algo que solo sirve para comparar contra un umbral.
	ocupado := int((usado*100 + total - 1) / total)

	return DiskStatus{Libre: libre, Total: total, Ocupado: ocupado}, nil
}

// checkDisk decide si se puede aceptar un mapa mas.
//
// Si la consulta del disco falla NO se bloquea la importacion: quedarse sin
// poder subir mapas porque una llamada al sistema devolvio error seria un
// remedio peor que la enfermedad. Se anota y se sigue; el margen absoluto que
// ya comprueba Import sigue protegiendo el caso extremo.
func (m *Maps) checkDisk() error {
	estado, err := m.Disk()
	if err != nil {
		m.log.Warn("no se pudo consultar el disco; se permite la importacion", "error", err)
		return nil
	}
	if estado.Lleno() {
		return domain.ErrNoDiskSpace
	}
	return nil
}
