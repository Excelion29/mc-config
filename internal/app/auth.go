package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Auth resuelve los casos de uso de autenticacion y sesion.
type Auth struct {
	users      UserRepo
	roles      RoleRepo
	sessions   SessionRepo
	hasher     Hasher
	tokens     TokenGenerator
	audit      *Audit
	clock      Clock
	sessionTTL time.Duration
	log        *slog.Logger
}

func NewAuth(
	users UserRepo,
	roles RoleRepo,
	sessions SessionRepo,
	hasher Hasher,
	tokens TokenGenerator,
	audit *Audit,
	clock Clock,
	sessionTTL time.Duration,
	log *slog.Logger,
) *Auth {
	return &Auth{
		users: users, roles: roles, sessions: sessions, hasher: hasher, tokens: tokens,
		audit: audit, clock: clock, sessionTTL: sessionTTL, log: log,
	}
}

func NormalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// CreateUser da de alta un usuario del panel.
func (a *Auth) CreateUser(ctx context.Context, email, password string, roleID int64) (*domain.User, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return nil, domain.ErrEmptyEmail
	}
	if len(password) < domain.MinPasswordLength {
		return nil, domain.ErrPasswordTooShort
	}

	// El rol tiene que existir: la clave foranea lo impediria igualmente, pero
	// asi el error es del dominio y no un mensaje del motor (D-14).
	role, err := a.roles.ByID(ctx, roleID)
	if err != nil {
		return nil, err
	}

	hash, err := a.hasher.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("hasheando contrasena: %w", err)
	}

	u := &domain.User{
		Email:        email,
		PasswordHash: hash,
		RoleID:       role.ID,
		Role:         role,
		Active:       true,
		CreatedAt:    a.clock(),
	}

	id, err := a.users.Create(ctx, u)
	if err != nil {
		return nil, err
	}
	u.ID = id
	return u, nil
}

// Login valida credenciales y abre una sesion.
//
// Devuelve siempre ErrInvalidCredentials ante correo inexistente o contrasena
// incorrecta: distinguirlos permitiria averiguar que cuentas existen.
func (a *Auth) Login(ctx context.Context, email, password, ip string) (*domain.Session, *domain.User, error) {
	email = NormalizeEmail(email)

	u, err := a.users.ByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		// Se gasta el mismo tiempo que en un login real. Sin esto, la
		// diferencia de latencia revela que correos estan registrados.
		a.hasher.Verify(decoyHash, password)
		a.audit.Record(ctx, nil, email, domain.ActionLoginFailed, "usuario inexistente", ip)
		return nil, nil, domain.ErrInvalidCredentials
	}
	if err != nil {
		return nil, nil, err
	}

	if !a.hasher.Verify(u.PasswordHash, password) {
		a.audit.Record(ctx, u, email, domain.ActionLoginFailed, "contrasena incorrecta", ip)
		return nil, nil, domain.ErrInvalidCredentials
	}
	if !u.Active {
		a.audit.Record(ctx, u, email, domain.ActionLoginFailed, "usuario desactivado", ip)
		return nil, nil, domain.ErrUserDisabled
	}

	token, err := a.tokens.New()
	if err != nil {
		return nil, nil, fmt.Errorf("generando token: %w", err)
	}

	now := a.clock()
	s := &domain.Session{
		Token:     token,
		UserID:    u.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(a.sessionTTL),
	}
	if err := a.sessions.Create(ctx, s); err != nil {
		return nil, nil, err
	}

	a.audit.Record(ctx, u, u.Email, domain.ActionLogin, "", ip)
	return s, u, nil
}

// Logout cierra la sesion.
func (a *Auth) Logout(ctx context.Context, token string, actor *domain.User, ip string) error {
	if actor != nil {
		a.audit.Record(ctx, actor, actor.Email, domain.ActionLogout, "", ip)
	}
	return a.sessions.Delete(ctx, token)
}

// UserFromSession resuelve el usuario a partir del token de cookie.
//
// Una sesion caducada, de un usuario borrado o desactivado se borra sobre la
// marcha: desactivar a alguien tiene que echarlo ya, no cuando caduque.
func (a *Auth) UserFromSession(ctx context.Context, token string) (*domain.User, error) {
	if token == "" {
		return nil, domain.ErrNotFound
	}

	s, err := a.sessions.ByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if s.Expired(a.clock()) {
		a.sessions.Delete(ctx, token)
		return nil, domain.ErrNotFound
	}

	u, err := a.users.ByID(ctx, s.UserID)
	if err != nil {
		a.sessions.Delete(ctx, token)
		return nil, err
	}
	if !u.Active {
		a.sessions.Delete(ctx, token)
		return nil, domain.ErrUserDisabled
	}

	return u, nil
}

// EnsureSuperuser garantiza el rol raiz y su unica cuenta.
//
// Es idempotente y no toca nada si ya hay un superusuario: el .env no puede
// recrear ni pisar cuentas por accidente. Todo lo demas -otros roles, otros
// usuarios- se crea desde el panel.
func (a *Auth) EnsureSuperuser(ctx context.Context, email, password string) error {
	role, err := a.EnsureRootRole(ctx)
	if err != nil {
		return err
	}

	n, err := a.roles.CountUsersByCode(ctx, domain.RoleCodeSuperuser)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	if email == "" || password == "" {
		a.log.Warn("no hay superusuario y no se configuro uno; " +
			"define MCVPS_SUPERUSER_EMAIL y MCVPS_SUPERUSER_PASSWORD para poder entrar")
		return nil
	}

	u, err := a.CreateUser(ctx, email, password, role.ID)
	if err != nil {
		return fmt.Errorf("creando el superusuario: %w", err)
	}

	a.log.Info("superusuario creado", "email", u.Email)
	a.audit.Record(ctx, u, u.Email, domain.ActionAdminSeeded, "arranque inicial", "sistema")
	return nil
}

// PurgeSessions limpia sesiones caducadas.
func (a *Auth) PurgeSessions(ctx context.Context) (int64, error) {
	return a.sessions.DeleteExpired(ctx, a.clock())
}

// decoyHash es un bcrypt valido de una contrasena cualquiera. Solo existe para
// consumir tiempo cuando el correo no existe: el resultado se descarta siempre,
// nunca concede acceso.
const decoyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
