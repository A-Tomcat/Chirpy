package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

/*
to build the Server:
go build -o out && ./out
*/

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerShowMetrics(w http.ResponseWriter, r *http.Request) {
	count := cfg.fileserverHits.Load()
	message := fmt.Sprintf("Hits: %d\n", count)
	w.Write([]byte(message))
}

func (cfg *apiConfig) handlerResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	message := fmt.Sprintln("Hits reset to 0.")
	w.Write([]byte(message))
}

func handler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte("OK\n"))
	if err != nil {
		return
	}
}

func main() {
	const filepathRoot = "."
	cfg := apiConfig{}
	cfg.fileserverHits.Store(0)
	sMux := http.NewServeMux()
	newServer := http.Server{
		Handler: sMux,
		Addr:    ":8080",
	}
	fServer := http.FileServer(http.Dir(filepathRoot))
	fServer = http.StripPrefix("/app", fServer)
	sMux.Handle("/app/", cfg.middlewareMetricsInc(fServer))
	sMux.HandleFunc("/reset", cfg.handlerResetMetrics)
	sMux.HandleFunc("/metrics", cfg.handlerShowMetrics)
	sMux.HandleFunc("/healthz", handler)

	newServer.ListenAndServe()
}
