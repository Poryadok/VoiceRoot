package httpserver

import (
	"net/http"

	"voice/backend/pkg/runtimeconfig"
)

// ApplyHTTPServerTimeouts sets ReadHeaderTimeout, ReadTimeout, WriteTimeout, and
// IdleTimeout on srv from runtime env (see runtimeconfig.HTTPServerTimeoutsFromEnv).
func ApplyHTTPServerTimeouts(srv *http.Server) {
	if srv == nil {
		return
	}
	t := runtimeconfig.HTTPServerTimeoutsFromEnv()
	srv.ReadHeaderTimeout = t.ReadHeader
	srv.ReadTimeout = t.Read
	srv.WriteTimeout = t.Write
	srv.IdleTimeout = t.Idle
}
