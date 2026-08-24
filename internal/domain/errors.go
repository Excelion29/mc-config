package domain

import "errors"

// Errores del dominio. Las capas de arriba comparan con errors.Is en lugar de
// inspeccionar mensajes de la base de datos, para que un cambio de motor no
// rompa la logica (D-14).
//
// El texto va en espanol porque parte de estos mensajes llegan al usuario.
var (
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
	ErrDuplicatePlayer = errors.New("ese gamertag ya esta en la lista")
	ErrEmptyGamertag   = errors.New("el gamertag es obligatorio")
)
