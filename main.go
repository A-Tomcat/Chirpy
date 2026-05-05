package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/internal/database"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

/*
cmd get to folder:
r

to build the Server:
go build -o out && ./out

to start the database:
sudo service postgresql start
password if needed: lulatsch12
enter the shell:
sudo -u postgres psql
postgres password = pass
[changed with ALTER USER postgres WITH PASSWORD 'pass';]


goose -dir sql/schema postgres "postgres://atomcat:lulatsch12@localhost:5432/chirpy?sslmode=disable" up
directly connect to DB :
psql "postgres://atomcat:lulatsch12@localhost:5432/chirpy"
if user not authentificated:
sudo -u postgres psql -d chirpy
GRANT ALL ON SCHEMA public TO atomcat;

go get github.com/google/uuid
go get github.com/lib/pq
go get github.com/joho/godotenv
*/

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	dev            string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
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
	if cfg.dev != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden")
		return
	}
	err := cfg.dbQueries.Reset(context.Background())
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	message := fmt.Sprintln("Hits reset to 0.")
	w.Write([]byte(message))
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

func handler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte("OK\n"))
	if err != nil {
		return
	}
}

func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type Params struct {
		Email string `json:"email"`
	}
	params := Params{}
	if err := readBody(r, &params); err != nil {
		respondWithError(w, 400, err.Error())
	}
	user, err := cfg.dbQueries.CreateUser(context.Background(), params.Email)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	mUser := User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	respondWithJson(w, http.StatusCreated, mUser)
}

func readBody(r *http.Request, params interface{}) error {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &params); err != nil {
		return err
	}
	return nil
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Body    string
		User_id uuid.UUID
	}
	params := Params{}
	if err := readBody(r, &params); err != nil {
		respondWithError(w, 400, err.Error())
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
	newBody := cleanProfane(params.Body, profanities)
	chirpParams := database.CreateChirpParams{
		Body:   newBody,
		UserID: params.User_id,
	}
	chirp, err := cfg.dbQueries.CreateChirp(context.Background(), chirpParams)
	if err != nil {
		respondWithError(w, 400, err.Error())
	}
	newChirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	respondWithJson(w, http.StatusCreated, newChirp)
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	type returnParams struct {
		Chirps []Chirp
	}
	chirps, err := cfg.dbQueries.GetChirps(context.Background())
	if err != nil {
		respondWithError(w, 400, err.Error())
	}
	rChirps := []Chirp{}
	for _, c := range chirps {
		chirp := Chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		}
		rChirps = append(rChirps, chirp)
	}
	respondWithJson(w, http.StatusOK, returnParams{
		Chirps: rChirps,
	})
}

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("couldn't load .env")
	}
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL is not set")
	}
	dev := os.Getenv("PLATFORM")
	const filepathRoot = "."
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	cfg := apiConfig{
		dbQueries: database.New(db),
		dev:       dev,
	}
	cfg.fileserverHits.Store(0)
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

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
	sMux.HandleFunc("POST /api/users", cfg.handlerCreateUser)
	sMux.HandleFunc("POST /api/chirps", cfg.handlerCreateChirp)
	sMux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)

	log.Fatal(newServer.ListenAndServe())
}
