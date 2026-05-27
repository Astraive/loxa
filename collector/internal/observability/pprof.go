package observability

import (
	"log"
	"net/http"
	"net/http/pprof"
	"strings"
)

// RegisterAndServePprof starts a dedicated pprof HTTP server on addr (e.g. ":6060").
func RegisterAndServePprof(addr string) {
	if strings.HasPrefix(addr, "0.0.0.0") {
		addr = "127.0.0.1" + addr[7:]
	} else if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go func() {
		log.Printf("pprof listening on %s", addr)
		_ = http.ListenAndServe(addr, mux)
	}()
}
