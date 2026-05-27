package httpserver

import (
	"net"
	"net/http"
	"net/http/pprof"
)

// localhostOnly wraps an http.HandlerFunc so it only responds to requests
// from 127.0.0.1 or ::1. Remote requests receive 403 Forbidden.
func localhostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil || (!ip.IsLoopback()) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func registerPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", localhostOnly(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", localhostOnly(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", localhostOnly(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", localhostOnly(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", localhostOnly(pprof.Trace))
}
