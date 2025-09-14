package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) resetCount(wr http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Swap(0)
	wr.Header().Add("Content-Type", "text/plain")
	wr.WriteHeader(200)
	wr.Write([]byte("Reset applied"))
}

func (cfg *apiConfig) countRequests(wr http.ResponseWriter, req *http.Request) {
	hits := fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())
	wr.Header().Add("Content-Type", "text/plain")
	wr.Write([]byte(hits))
}

func main() {

	apiCfg := apiConfig{}
	serverMux := http.NewServeMux()
	server := http.Server{
		Handler: serverMux,
		Addr:    ":8080",
	}

	fs := http.StripPrefix("/app/", http.FileServer(http.Dir("./public")))
	// appHandler := http.StripPrefix("/app", fs)
	serverMux.Handle("/app/", apiCfg.middlewareMetricsInc(fs))

	assetsFileServer := http.FileServer(http.Dir("./assets"))
	assetsHandler := http.StripPrefix("/app/assets", assetsFileServer)
	serverMux.Handle("/app/assets/", apiCfg.middlewareMetricsInc(assetsHandler))
	serverMux.HandleFunc("GET /api/healthz", func(wr http.ResponseWriter, req *http.Request) {
		wr.WriteHeader(200)
		wr.Header().Add("Content-Type", "text/plain; charset=utf-8")
		wr.Write([]byte("OK"))
	})
	serverMux.HandleFunc("GET /api/metrics", apiCfg.countRequests)
	serverMux.HandleFunc("POST /api/reset", apiCfg.resetCount)

	server.ListenAndServe()
}
