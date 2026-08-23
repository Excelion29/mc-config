package app

// Paging es lo que pide quien consulta: que pagina y de que tamano.
//
// Vive en la capa de casos de uso y no en cada adaptador porque la regla de
// "que es una pagina valida" es una sola para toda la aplicacion. Si cada
// pantalla la resolviera por su cuenta, acabarian discrepando.
type Paging struct {
	Page int
	Size int
}

// Normalize deja la peticion en un estado utilizable venga como venga.
//
// Los valores llegan de la URL, que escribe cualquiera: "?p=-4" o "?p=abc"
// tienen que dar la primera pagina, no un error ni un OFFSET negativo. El
// tope de tamano existe para que nadie pueda pedir "?n=100000" y obligar al
// panel a construir esa tabla entera.
func (p Paging) Normalize(porDefecto, maximo int) Paging {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Size < 1 || p.Size > maximo {
		p.Size = porDefecto
	}
	return p
}

func (p Paging) Offset() int { return (p.Page - 1) * p.Size }

// PageInfo es lo que necesita la plantilla para pintar el paginador.
//
// Se calcula aqui y no en el HTML a proposito: "cuantas paginas hay" es una
// division con un caso limite -el resto- y esa clase de cosas no pertenecen a
// una plantilla.
type PageInfo struct {
	// Total son las filas que cumplen el filtro, no las de la tabla entera.
	Total int
	Page  int
	Size  int
}

func NewPageInfo(p Paging, total int) PageInfo {
	return PageInfo{Total: total, Page: p.Page, Size: p.Size}
}

func (p PageInfo) Pages() int {
	if p.Size <= 0 || p.Total <= 0 {
		return 1
	}
	n := p.Total / p.Size
	if p.Total%p.Size != 0 {
		n++
	}
	return n
}

func (p PageInfo) HasPrev() bool { return p.Page > 1 }
func (p PageInfo) HasNext() bool { return p.Page < p.Pages() }
func (p PageInfo) Prev() int     { return p.Page - 1 }
func (p PageInfo) Next() int     { return p.Page + 1 }

// Multiple indica si merece la pena ensenar el paginador. Con una sola pagina
// no aporta nada y solo mete ruido.
func (p PageInfo) Multiple() bool { return p.Pages() > 1 }

// From y To son la primera y la ultima fila de esta pagina, para poder decir
// "26-50 de 312" en vez de solo un numero de pagina suelto, que no responde a
// la pregunta que se hace quien mira: cuanto queda.
func (p PageInfo) From() int {
	if p.Total == 0 {
		return 0
	}
	return (p.Page-1)*p.Size + 1
}

func (p PageInfo) To() int {
	to := p.Page * p.Size
	if to > p.Total {
		return p.Total
	}
	return to
}
