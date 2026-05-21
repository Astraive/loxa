package observability

import (
	"log"
	"net/http"
	"net/http/pprof"
)

// RegisterAndServePprof starts a dedicated pprof HTTP server on addr (e.g. ":6060").
func RegisterAndServePprof(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go func() {
		log.Printf("pprof listening on %s", addr)
		http.ListenAndServe(addr, mux)
	}()
}
