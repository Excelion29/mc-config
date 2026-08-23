package domain

import "time"

// Session es una sesion abierta del panel.
//
// Se eligen sesiones en cookie frente a JWT por dos razones: se revocan al
// instante borrando la fila, y el token nunca pasa por JavaScript. Un JWT
// valido sigue siendolo aunque desactives al usuario, hasta que caduca.
type Session struct {
	Token     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Expired indica si la sesion ya no sirve en el instante dado.
func (s Session) Expired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}
