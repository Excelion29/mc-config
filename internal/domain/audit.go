package domain

import "time"

// Action es una accion registrable en el log.
//
// Por D-08 el registro existe desde el primer dia: si un operador cambia de
// mapa y desconecta a los que estaban jugando, hay que poder saber quien fue.
type Action string

const (
	ActionLogin       Action = "login"
	ActionLoginFailed Action = "login_failed"
	ActionLogout      Action = "logout"
	ActionAdminSeeded Action = "initial_admin_created"

	ActionUserCreated     Action = "user_created"
	ActionUserEnabled     Action = "user_enabled"
	ActionUserDisabled    Action = "user_disabled"
	ActionUserRoleChanged Action = "user_role_changed"

	ActionRoleCreated Action = "role_created"
	ActionRoleUpdated Action = "role_updated"
	ActionRoleDeleted Action = "role_deleted"

	ActionWorldImported    Action = "map_imported"
	ActionWorldCreated     Action = "world_created"
	ActionWorldUpdated     Action = "world_updated"
	ActionResourceCreated  Action = "pack_created"
	ActionResourceUpdated  Action = "pack_updated"
	ActionResourceDeleted  Action = "pack_deleted"
	ActionResourceAssigned Action = "pack_assigned"

	// Cambiar el modo de autenticacion es de las acciones mas delicadas que
	// existen en el panel: decide si hace falta cuenta comprada para entrar.
	ActionAuthModeChanged      Action = "auth_mode_changed"
	ActionPluginVersionChanged Action = "plugin_version_changed"
	ActionPluginsInstalled     Action = "plugins_installed"
	ActionWorldDeleted         Action = "map_deleted"

	ActionInstanceCreated Action = "instance_created"
	ActionInstanceStarted Action = "instance_started"
	ActionInstanceStopped Action = "instance_stopped"
	ActionInstanceDeleted Action = "instance_deleted"

	ActionPlayerAdded    Action = "player_added"
	ActionPlayerUpdated  Action = "player_updated"
	ActionPlayerRemoved  Action = "player_removed"
	ActionPlayerEnabled  Action = "player_enabled"
	ActionPlayerDisabled Action = "player_disabled"
	ActionPlayerOp       Action = "player_op_changed"
)

// LogEntry es una accion ya ocurrida.
//
// UserEmail se guarda desnormalizado a proposito: si el usuario se borra, el
// registro de lo que hizo debe sobrevivir. Un log que desaparece con el usuario
// no sirve para auditar nada.
type LogEntry struct {
	ID        int64
	UserID    *int64 // nil si el actor no existia (login con correo inventado)
	UserEmail string
	Action    Action
	Detail    string
	IP        string
	CreatedAt time.Time
}
