package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// SettingsRepo guarda los ajustes globales del panel.
type SettingsRepo interface {
	Get(ctx context.Context, clave, porDefecto string) (string, error)
	Set(ctx context.Context, clave, valor string) error
}

// Access decide quien autentica a los jugadores (D-17).
//
// Es lo que en la interfaz se ofrece como "dejar entrar a cuentas no premium".
// Detras hay un modo de servidor y dos plugins, pero quien administra no tiene
// por que saber eso: pide una capacidad, no una configuracion.
type Access struct {
	settings  SettingsRepo
	versiones PluginVersionRepo
	instances *Instances
	audit     *Audit
	log       *slog.Logger
}

func NewAccess(settings SettingsRepo, versiones PluginVersionRepo,
	instances *Instances, audit *Audit, log *slog.Logger) *Access {

	return &Access{settings: settings, versiones: versiones,
		instances: instances, audit: audit, log: log}
}

// VersionesDePlugins da las versiones elegidas desde el panel.
//
// Ante un fallo devuelve el mapa vacio, y eso significa "manda el codigo": la
// version verificada a mano. Es el lado seguro por el que caer.
func (a *Access) VersionesDePlugins(ctx context.Context) map[string]PluginVersion {
	v, err := a.versiones.All(ctx)
	if err != nil {
		a.log.Error("no se pudieron leer las versiones de los complementos; se usan las de fabrica",
			"error", err)
		return nil
	}
	return v
}

// CambiarVersion pone otra version de un complemento, sin desplegar.
//
// La de fabrica sigue en el codigo y es la que manda mientras nadie toque nada.
// Esto existe porque atarse a un despliegue para subir un plugin significa no
// subirlo nunca, y un plugin viejo tambien es un problema.
//
// Con url vacia se vuelve a la de fabrica.
func (a *Access) CambiarVersion(ctx context.Context, actor *domain.User,
	pluginID, url, ip string) error {

	if !actor.Can(domain.PermServerOperate) {
		return domain.ErrForbidden
	}

	actuales := a.instances.PluginsPara(domain.EditionJava, domain.AuthOffline)
	anterior, ok := buscarPlugin(actuales, pluginID)
	if !ok {
		return domain.ErrPluginDesconocido
	}

	url = strings.TrimSpace(url)
	if url == "" {
		if err := a.versiones.Clear(ctx, pluginID); err != nil {
			return err
		}
		a.retirarDeTodas(ctx, anterior.File)
		a.audit.Record(ctx, actor, actor.Email, domain.ActionPluginVersionChanged,
			anterior.Name+": de fabrica", ip)
		return nil
	}

	archivo, err := domain.ArchivoDeJar(url)
	if err != nil {
		return err
	}

	if err := a.versiones.Set(ctx, PluginVersion{
		PluginID: pluginID, URL: url, File: archivo,
	}, actor.ID); err != nil {
		return err
	}

	// El .jar viejo se va. Dos versiones del mismo plugin en plugins/ se cargan
	// LAS DOS, y el servidor arranca peleandose consigo mismo sin decir por
	// que. Se hace despues de guardar: si algo falla, es mejor quedarse sin el
	// viejo que con los dos.
	if archivo != anterior.File {
		a.retirarDeTodas(ctx, anterior.File)
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionPluginVersionChanged,
		anterior.Name+": "+archivo, ip)
	a.log.Warn("version de complemento cambiada desde el panel",
		"plugin", pluginID, "archivo", archivo, "por", actor.Email)
	return nil
}

// retirarDeTodas borra un .jar de todas las instancias de Java.
//
// No corta si falla: lo importante -la version nueva- ya esta guardado, y el
// arranque instalara la que toque. Un .jar viejo que se resiste a borrarse es
// un problema, pero no uno que deba impedir el cambio.
func (a *Access) retirarDeTodas(ctx context.Context, archivo string) {
	javas, _, err := a.instanciasJava(ctx)
	if err != nil {
		a.log.Warn("no se pudo listar las instancias para retirar el complemento viejo",
			"error", err)
		return
	}
	for _, inst := range javas {
		if err := a.instances.RetirarPlugin(inst, archivo); err != nil {
			a.log.Warn("quedo un complemento viejo sin borrar",
				"instancia", inst.Name, "archivo", archivo, "error", err)
		}
	}
}

func buscarPlugin(lista []Plugin, id string) (Plugin, bool) {
	for _, p := range lista {
		if p.ID == id {
			return p, true
		}
	}
	return Plugin{}, false
}

// Mode devuelve el modo vigente.
//
// Ante cualquier duda devuelve el modo NORMAL: si no se puede leer el ajuste,
// mas vale un servidor que solo acepta cuentas compradas que uno abierto a que
// cualquiera use el nombre que quiera.
func (a *Access) Mode(ctx context.Context) domain.AuthMode {
	v, err := a.settings.Get(ctx, domain.SettingAuthMode, string(domain.AuthOnline))
	if err != nil {
		a.log.Error("no se pudo leer el modo de autenticacion; se usa el seguro", "error", err)
		return domain.AuthOnline
	}

	modo := domain.AuthMode(v)
	if !modo.Valid() {
		a.log.Error("modo de autenticacion desconocido; se usa el seguro", "valor", v)
		return domain.AuthOnline
	}
	return modo
}

// Estado describe la situacion del acceso, para poder explicarla en pantalla.
type Estado struct {
	Mode domain.AuthMode
	// Requeridos son los plugins que hace falta tener para el modo sin
	// conexion.
	Requeridos []Plugin
	// Faltan son los que NO estan instalados en la instancia.
	Faltan []Plugin
	// FaltanEsenciales son los que ademas PROTEGEN. Mientras haya alguno, el
	// modo sin conexion no se puede activar.
	//
	// Van aparte de Faltan porque no es lo mismo quedarse sin skins que sin
	// contrasena: lo primero es una molestia y lo segundo es una puerta
	// abierta.
	FaltanEsenciales []Plugin
	// Instancias son TODOS los servidores donde tiene que estar puesto.
	//
	// Son todos y no el primero a proposito: el modo es global y se aplica a
	// cada servidor que arranque. Comprobar solo uno diria "listo" mientras
	// otro arranca abierto y sin nada que lo proteja.
	Instancias []string
	// Filas es la lista para pintar, con el estado ya resuelto de cada uno.
	//
	// Se calcula aqui y no en la plantilla porque "esta este plugin en la lista
	// de los que faltan" es una busqueda, y una plantilla que busca acaba
	// siendo ilegible o, peor, equivocandose en silencio.
	Filas []PluginRow
}

// PluginRow es un complemento con su estado.
type PluginRow struct {
	Plugin
	Puesto bool
}

// Listo indica si se puede activar el modo sin conexion.
//
// Solo miran los ESENCIALES. Que falte SkinsRestorer no puede impedir abrir el
// acceso: sin el se pierden las skins, no la seguridad.
func (e Estado) Listo() bool {
	return len(e.FaltanEsenciales) == 0 && len(e.Instancias) > 0
}

// Donde nombra los servidores afectados, para poder decirlo en pantalla.
func (e Estado) Donde() string { return strings.Join(e.Instancias, ", ") }

// Estado dice en que punto esta el acceso y que falta.
//
// Se comprueba contra la instancia de Java que exista, porque los plugins viven
// en su carpeta. Sin instancia Java no hay nada que comprobar ni nada que
// activar.
func (a *Access) Estado(ctx context.Context, actor *domain.User) (Estado, error) {
	if !actor.Can(domain.PermServerOperate) {
		return Estado{}, domain.ErrForbidden
	}

	est := Estado{Mode: a.Mode(ctx)}

	javas, plugins, err := a.instanciasJava(ctx)
	if err != nil || len(javas) == 0 {
		return est, err
	}
	est.Requeridos = plugins

	// Un plugin cuenta como puesto solo si esta en TODAS. Basta que falte en
	// una para que abrir el acceso deje ese servidor sin proteger.
	faltaEn := map[string]int{}
	for _, inst := range javas {
		est.Instancias = append(est.Instancias, inst.Name)
		for _, p := range a.instances.PluginsQueFaltan(inst, plugins) {
			faltaEn[p.File]++
		}
	}

	for _, p := range plugins {
		puesto := faltaEn[p.File] == 0
		est.Filas = append(est.Filas, PluginRow{Plugin: p, Puesto: puesto})
		if puesto {
			continue
		}
		est.Faltan = append(est.Faltan, p)
		if p.Esencial {
			est.FaltanEsenciales = append(est.FaltanEsenciales, p)
		}
	}
	return est, nil
}

// SetMode cambia el modo de autenticacion.
//
// Pasar a sin conexion EXIGE que los plugins esten instalados. No es una
// comprobacion de cortesia: sin ellos ese modo deja el servidor abierto a que
// cualquiera entre con el nombre que quiera, incluido el de quien administra
// (D-07). Por eso se niega en vez de avisar.
func (a *Access) SetMode(ctx context.Context, actor *domain.User, modo domain.AuthMode, ip string) error {
	if !actor.Can(domain.PermServerOperate) {
		return domain.ErrForbidden
	}
	if !modo.Valid() {
		return domain.ErrInvalidSettings
	}

	if modo.SinConexion() {
		est, err := a.Estado(ctx, actor)
		if err != nil {
			return err
		}
		if !est.Listo() {
			return domain.ErrPluginsMissing
		}
	}

	if err := a.settings.Set(ctx, domain.SettingAuthMode, string(modo)); err != nil {
		return err
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionAuthModeChanged, modo.Label(), ip)
	a.log.Warn("modo de autenticacion cambiado", "modo", modo, "por", actor.Email)
	return nil
}

// PrepararPlugins descarga e instala lo que hace falta para el modo sin
// conexion, sin activarlo todavia.
//
// Va en dos pasos a proposito: primero se instala con el servidor todavia en
// modo normal -sin riesgo ninguno- y solo despues se cambia el interruptor. Al
// reves habria un rato con el servidor abierto y los plugins a medio poner.
func (a *Access) PrepararPlugins(ctx context.Context, actor *domain.User, ip string) error {
	if !actor.Can(domain.PermServerOperate) {
		return domain.ErrForbidden
	}

	javas, plugins, err := a.instanciasJava(ctx)
	if err != nil {
		return err
	}
	if len(javas) == 0 {
		return domain.ErrNoJavaInstance
	}

	// En todas, no solo en la primera. Y si una falla se corta: dejarlo a
	// medias y decir que fue bien es peor que no haberlo intentado.
	for _, inst := range javas {
		if err := a.instances.InstalarPlugins(ctx, inst, plugins); err != nil {
			return err
		}
		a.audit.Record(ctx, actor, actor.Email, domain.ActionPluginsInstalled, inst.Name, ip)
	}
	return nil
}

// instanciasJava devuelve TODOS los servidores de Java y los plugins que
// necesitarian en modo sin conexion.
//
// Se pregunta siempre por AuthOffline y no por el modo vigente: la pantalla
// tiene que poder decir que hara falta ANTES de activarlo. En modo normal la
// lista seria vacia y no habria nada que ensenar.
func (a *Access) instanciasJava(ctx context.Context) ([]*domain.Instance, []Plugin, error) {
	lista, err := a.instances.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	var javas []*domain.Instance
	for i := range lista {
		if lista[i].Edition == domain.EditionJava {
			javas = append(javas, &lista[i])
		}
	}
	if len(javas) == 0 {
		return nil, nil, nil
	}
	return javas, a.instances.PluginsPara(domain.EditionJava, domain.AuthOffline), nil
}
