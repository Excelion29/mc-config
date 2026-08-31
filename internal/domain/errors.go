package domain

import "errors"

// Errores del dominio. Las capas de arriba comparan con errors.Is en lugar de
// inspeccionar mensajes de la base de datos, para que un cambio de motor no
// rompa la logica (D-14).
//
// El texto va en espanol porque parte de estos mensajes llegan al usuario.
var (
	// ErrPluginsMissing: se intento activar el modo sin conexion sin tener los
	// plugins puestos. Se rechaza y no se avisa: sin ellos ese modo deja el
	// servidor abierto a que cualquiera use el nombre de otro.
	ErrPluginsMissing = errors.New("faltan plugins para ese modo")
	// ErrPluginDesconocido: se intento cambiar la version de algo que el panel
	// no gestiona. La lista la decide el codigo, no quien manda el formulario.
	ErrPluginDesconocido = errors.New("ese complemento no existe")
	// ErrJarInvalido: el enlace no lleva a un .jar. Instalar otra cosa en
	// plugins/ no da error: el servidor la ignora y el plugin no esta.
	ErrJarInvalido = errors.New("el enlace tiene que apuntar a un archivo .jar")
	// ErrPluginsUnavailable: el panel no tiene instalador de plugins
	// configurado.
	ErrPluginsUnavailable = errors.New("la instalacion de plugins no esta disponible")
	// ErrNoJavaInstance: no hay ningun servidor de Java donde instalar nada.
	ErrNoJavaInstance = errors.New("no hay ningun servidor de Java")
	// ErrJavaNameNotFound: Mojang no conoce ese nombre de Java. Casi siempre
	// es una errata, y decirlo al darlo de alta evita el "no puedo entrar" de
	// dentro de una semana.
	ErrJavaNameNotFound = errors.New("esa cuenta de Java no existe")
	// ErrInvalidIconURL: la portada tiene que ser https. El panel se sirve por
	// TLS y una imagen http la bloquea el navegador por contenido mixto: se
	// veria rota sin decir por que, asi que se rechaza al escribirla.
	ErrInvalidIconURL = errors.New("la portada debe empezar por https://")
	// ErrInvalidSettings: un ajuste fuera de los valores permitidos. Se
	// rechaza en vez de escribirlo: el servidor ignoraria el valor raro y se
	// comportaria de otra forma sin dar ningun error.
	ErrInvalidSettings = errors.New("ajustes invalidos")
	// ErrEmptyName: un mundo sin nombre no se puede listar ni elegir.
	ErrEmptyName = errors.New("el nombre es obligatorio")

	ErrNotFound           = errors.New("no encontrado")
	ErrDuplicateEmail     = errors.New("ya existe un usuario con ese correo")
	ErrInvalidCredentials = errors.New("correo o contrasena incorrectos")
	ErrUserDisabled       = errors.New("el usuario esta desactivado")
	ErrInvalidRole        = errors.New("rol invalido")
	ErrPasswordTooShort   = errors.New("la contrasena es demasiado corta")
	ErrEmptyEmail         = errors.New("el correo es obligatorio")
	ErrForbidden          = errors.New("no tienes permiso para esta accion")

	// Evita el escenario de quedarse sin ningun admin operativo.
	ErrSelfDisable = errors.New("no puedes desactivar tu propia cuenta")

	// Errores de RBAC.
	ErrRoleNotFound     = errors.New("el rol no existe")
	ErrDuplicateRole    = errors.New("ya existe un rol con ese codigo")
	ErrSystemRole       = errors.New("los roles del sistema no se pueden borrar")
	ErrAdminRoleLocked  = errors.New("el rol administrador tiene siempre todos los permisos")
	ErrRoleInUse        = errors.New("hay usuarios con ese rol; cambialos antes de borrarlo")
	ErrUnknownPermiso   = errors.New("permiso desconocido")
	ErrEmptyRoleCode    = errors.New("el codigo del rol es obligatorio")
	ErrSelfRoleDowngrade = errors.New("no puedes quitarte a ti mismo la gestion de usuarios")

	// Errores de la biblioteca de mapas (F2).
	ErrNoFile          = errors.New("no se recibio ningun archivo")
	ErrFileTooBig      = errors.New("el archivo supera el tamano maximo permitido")
	ErrNotAnArchive    = errors.New("el archivo no es un zip valido")
	ErrNotAWorld       = errors.New("el archivo no contiene un mundo de Minecraft")
	ErrUnsafePath      = errors.New("el archivo contiene rutas inseguras")
	ErrZipBomb         = errors.New("el archivo se expande demasiado al descomprimirse")
	ErrNoDiskSpace     = errors.New("no hay espacio suficiente en disco")
	ErrDuplicateWorld    = errors.New("ese mapa ya esta en la biblioteca")
	ErrWorldNotFound     = errors.New("el mapa no existe")

	// --- Paquetes de texturas ---
	ErrResourceNotFound = errors.New("el paquete no existe")
	// ErrResourceDuplicado: ya hay un paquete con ese mismo enlace. El enlace es
	// lo que identifica al paquete de verdad -el nombre lo pone quien lo
	// anade- y tener dos filas con el mismo acabaria en dos entradas que hay
	// que corregir por separado el dia que el autor lo mueva.
	ErrResourceDuplicado = errors.New("ya existe un paquete con ese enlace")
	// ErrResourceURLInvalida: el enlace no sirve. Solo https, porque ese archivo
	// acaba dentro del juego de cada persona que entra.
	ErrResourceURLInvalida = errors.New("el enlace del paquete no es valido")
	// ErrResourcePrincipalNoAsignado: se marco como activo un paquete que no esta en
	// la lista del mundo. Es un formulario incoherente, no una eleccion.
	ErrResourcePrincipalNoAsignado = errors.New("el paquete activo no esta asignado al mundo")
	// ErrDemasiadosRecursos: el mundo ya lleva el maximo. No es por espacio -el
	// panel solo guarda el enlace- sino por lo que se le pide a quien juega:
	// cada recurso que no se aplica solo es un enlace que alguien tiene que
	// abrir, descargar e instalar antes de entrar.
	ErrDemasiadosRecursos = errors.New("el mundo ya lleva demasiados recursos")
	// ErrResourceYaEnMundo: ese enlace ya esta en la lista del mundo.
	ErrResourceYaEnMundo = errors.New("el mundo ya lleva ese recurso")
	// ErrResourceNoAutomatico: se quiso aplicar solo un paquete cuyo enlace apunta
	// a una pagina y no al archivo. El cliente recibiria HTML esperando un zip.
	ErrResourceNoAutomatico = errors.New("ese enlace no apunta al archivo del paquete")
	ErrEditionMismatch = errors.New("la edicion del mapa no coincide con la de la instancia")

	// Jerarquia de roles.
	ErrSamePeer          = errors.New("no puedes gestionar a alguien de tu mismo nivel")
	ErrSuperuserLocked   = errors.New("el superusuario no lo puede gestionar nadie")
	ErrOnlyOneSuperuser  = errors.New("solo puede haber un superusuario")
	ErrRoleAboveYou      = errors.New("no puedes asignar ni editar un rol igual o superior al tuyo")
	ErrRoleLevelTooHigh  = errors.New("no puedes crear un rol de nivel igual o superior al tuyo")

	// Instancias de servidor (F3).
	ErrInstanceNotFound  = errors.New("la instancia no existe")
	ErrDuplicateInstance = errors.New("ya existe una instancia con ese nombre")
	ErrEmptyInstanceName = errors.New("el nombre de la instancia es obligatorio")
	ErrInstanceBusy      = errors.New("la instancia esta arrancando o parando; espera a que termine")
	ErrInstanceRunning   = errors.New("detén la instancia antes de borrarla")

	// Lista maestra de jugadores (F4).
	ErrPlayerNotFound  = errors.New("el jugador no existe")
	// ErrNeedsRestart: el cambio esta guardado, pero el servidor solo lo lee al
	// arrancar. No es un fallo: es algo que hay que contarle a quien lo hizo.
	ErrNeedsRestart = errors.New("hace falta reiniciar el servidor para que surta efecto")
	ErrDuplicatePlayer = errors.New("ese gamertag ya esta en la lista")
	ErrEmptyGamertag   = errors.New("el gamertag es obligatorio")
)
