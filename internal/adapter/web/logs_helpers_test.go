package web

import (
	"net/http"
	"strings"
)

// escritorFalso permite probar emitirEvento sin levantar un servidor.
type escritorFalso struct{ sb *strings.Builder }

func (e *escritorFalso) Header() http.Header       { return http.Header{} }
func (e *escritorFalso) Write(p []byte) (int, error) { return e.sb.Write(p) }
func (e *escritorFalso) WriteHeader(int)           {}
