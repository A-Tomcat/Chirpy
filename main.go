package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

/*
cmd get to folder:
cd git/workspace/A-Tomcat/Chirpy

to build the Server:
go build -o out && ./out

to start the database:
sudo service postgresql start
password if needed: lulatsch12
enter the shell:
sudo -u postgres psql
postgres password = pass
[changed with ALTER USER postgres WITH PASSWORD 'pass';]


goose -dir sql/schema postgres "postgres://postgres:pass@localhost:5432/chirpy?sslmode=disable" up

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
	w.Header().Set("Content-Type", "text/html;")
	message := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", count)
	w.Write([]byte(message))
}

func (cfg *apiConfig) handlerResetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	message := fmt.Sprintln("Hits reset to 0.")
	w.Write([]byte(message))
}

func handlerCheckChirp(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type Params struct {
		Body string `json:"body"`
	}
	type ReturnParams struct {
		CleanedBody string `json:"cleaned_body"`
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	params := Params{}
	if err = json.Unmarshal(data, &params); err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp exceeds character limit.")
		return
	}
	profanities := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}
	newMSG := cleanProfane(params.Body, profanities)
	respondWithJson(w, http.StatusOK, ReturnParams{
		CleanedBody: newMSG,
	})
	/*
		respondWithValid(w, http.StatusOK, true)
	*/
}

func cleanProfane(msg string, profanities []string) string {
	clean := ""
	words := strings.Fields(msg)
	for _, word := range words {
		for _, profanity := range profanities {
			if strings.ToLower(word) == profanity {
				word = "****"
				continue
			}
		}
		clean += word + " "
	}
	clean = strings.TrimSpace(clean)

	return clean
}

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
	return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJson(w, code, map[string]string{"error": msg})
}

func respondWithValid(w http.ResponseWriter, code int, valid bool) error {
	return respondWithJson(w, code, map[string]bool{"valid": valid})
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
	appServer := http.StripPrefix("/app", fServer)
	sMux.Handle("/app/", cfg.middlewareMetricsInc(appServer))
	sMux.HandleFunc("GET /api/healthz", handler)
	sMux.HandleFunc("POST /admin/reset", cfg.handlerResetMetrics)
	sMux.HandleFunc("GET /admin/metrics", cfg.handlerShowMetrics)
	sMux.HandleFunc("POST /api/validate_chirp", handlerCheckChirp)

	newServer.ListenAndServe()
}
