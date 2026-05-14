package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"main/internal/auth"
	"main/internal/database"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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


goose -dir sql/schema postgres "postgres://atomcat:lulatsch12@localhost:5432/chirpy?sslmode=disable" up
directly connect to DB :
psql "postgres://atomcat:lulatsch12@localhost:5432/chirpy"
if user not authentificated:
sudo -u postgres psql -d chirpy
GRANT ALL ON SCHEMA public TO atomcat;

go get github.com/google/uuid
go get github.com/lib/pq
go get github.com/joho/godotenv
go get github.com/alexedwards/argon2id
*/

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	dev            string
	secret         string
	polka_key      string
}

type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

const token_expire = 3600 * time.Second
const refresh_token_expirer = 5184000 * time.Second

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
		Email    string `json:"email"`
		Password string `json:"password"`
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
	pass, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 400, err.Error())
	}
	passParams := database.AddPasswordParams{
		ID:             user.ID,
		HashedPassword: pass,
	}
	if err := cfg.dbQueries.AddPassword(context.Background(), passParams); err != nil {
		respondWithError(w, 400, err.Error())
	}
	mUser := User{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	}
	respondWithJson(w, http.StatusCreated, mUser)
}

func readBody(r *http.Request, params interface{}) error {
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, params); err != nil {
		return err
	}
	return nil
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Body string
	}
	params := Params{}
	if err := readBody(r, &params); err != nil {
		respondWithError(w, 400, err.Error())
	}
	tokenString, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := auth.ValidateJWT(tokenString, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
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
	newBody := cleanProfane(params.Body, profanities)
	chirpParams := database.CreateChirpParams{
		Body:   newBody,
		UserID: id,
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
	u_id := r.URL.Query().Get("author_id")
	sortQ := r.URL.Query().Get("sort")
	var chirps []database.Chirp
	var err error
	if u_id == "" {
		chirps, err = cfg.dbQueries.GetChirps(context.Background())
		if err != nil {
			respondWithError(w, 400, err.Error())
		}
	} else {
		id, err := uuid.Parse(u_id)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, err.Error())
		}
		chirps, err = cfg.dbQueries.GetChirpyByUsers(context.Background(), id)
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
	if sortQ == "desc" {
		sort.Slice(rChirps, func(i, j int) bool {
			return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
		})
	}
	respondWithJson(w, http.StatusOK, rChirps)
}

func (cfg *apiConfig) handlerGetChirpByID(w http.ResponseWriter, r *http.Request) {
	idString := r.PathValue("chirpID")
	id, err := uuid.Parse(idString)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	chirp, err := cfg.dbQueries.GetChirpById(context.Background(), id)
	if err != nil {

		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	nchirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	respondWithJson(w, http.StatusOK, nchirp)
}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	params := Params{}
	err := readBody(r, &params)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	u, err := cfg.dbQueries.GetUserFromEmail(context.Background(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}
	ref_token_string := auth.MakeRefreshToken()
	ref_params := database.CreateRefreshTokenParams{
		Token:     ref_token_string,
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(refresh_token_expirer),
	}
	_, err = cfg.dbQueries.CreateRefreshToken(context.Background(), ref_params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}
	if v, err := auth.CheckPasswordHash(params.Password, u.HashedPassword); err == nil {
		if v {
			token, err := auth.MakeJWT(u.ID, cfg.secret, token_expire)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, err.Error())
				return
			}
			user := User{
				ID:           u.ID,
				CreatedAt:    u.CreatedAt,
				UpdatedAt:    u.UpdatedAt,
				Email:        u.Email,
				Token:        token,
				RefreshToken: ref_token_string,
				IsChirpyRed:  u.IsChirpyRed,
			}
			respondWithJson(w, http.StatusOK, user)

		} else {
			respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}
	} else {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type ReturnParams struct {
		Token string `json:"token"`
	}
	unclean := r.Header.Get("Authorization")
	if unclean == "" {
		respondWithError(w, http.StatusNotFound, "No Authorization Header exists.")
		return
	}
	token_string := strings.TrimPrefix(unclean, "Bearer ")
	token, err := cfg.dbQueries.GetRefreshToken(context.Background(), token_string)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if token.RevokedAt.Valid || token.ExpiresAt.Before(time.Now()) {
		respondWithError(w, http.StatusUnauthorized, "Token has expired or been revoked.")
		return
	}
	u_id, err := cfg.dbQueries.GetUserFromRefreshToken(context.Background(), token_string)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	new_token, err := auth.MakeJWT(u_id, cfg.secret, token_expire)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}
	respondWithJson(w, http.StatusOK, ReturnParams{
		Token: new_token})

}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	token_string, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 400, err.Error())
	}
	if err := cfg.dbQueries.RevokeRefreshToken(context.Background(), token_string); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJson(w, http.StatusNoContent, nil)
}

func (cfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, r *http.Request) {
	token_string, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	u_id, err := auth.ValidateJWT(token_string, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	type Params struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	params := Params{}
	err = readBody(r, &params)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	new_pass, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	userParams := database.UpdateUserParams{
		ID:             u_id,
		Email:          params.Email,
		HashedPassword: new_pass,
	}
	u, err := cfg.dbQueries.UpdateUser(context.Background(), userParams)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJson(w, 200, User{
		ID:          u.ID,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		Email:       u.Email,
		IsChirpyRed: u.IsChirpyRed,
	})
}

func (cfg *apiConfig) handleDeleteChirp(w http.ResponseWriter, r *http.Request) {
	token_string, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	u_id, err := auth.ValidateJWT(token_string, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}
	idString := r.PathValue("chirpID")
	c_id, err := uuid.Parse(idString)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	chirp, err := cfg.dbQueries.GetChirpById(context.Background(), c_id)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}
	if chirp.UserID != u_id {
		respondWithError(w, http.StatusForbidden, "No permission for this action.")
		return
	}
	err = cfg.dbQueries.DeleteChirp(context.Background(), c_id)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJson(w, http.StatusNoContent, "Chirp successfully deleted.")
}

func (cfg *apiConfig) handlerUpdgradeUser(w http.ResponseWriter, r *http.Request) {
	api, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	if api != cfg.polka_key {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized. Access denied")
		return
	}
	type Params struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}
	params := Params{}
	err = readBody(r, &params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
	}
	if params.Event != "user.upgraded" {
		respondWithJson(w, http.StatusNoContent, nil)
		return
	}
	err = cfg.dbQueries.UpgradeUser(context.Background(), params.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJson(w, http.StatusNoContent, nil)
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
	secretbase := os.Getenv("secret")
	cfg := apiConfig{
		dbQueries: database.New(db),
		dev:       dev,
		secret:    secretbase,
	}
	polka_key := os.Getenv("POLKA_KEY")
	cfg.polka_key = polka_key
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
	sMux.HandleFunc("GET /admin/metrics", cfg.handlerShowMetrics)
	sMux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)
	sMux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handlerGetChirpByID)
	sMux.HandleFunc("POST /admin/reset", cfg.handlerResetMetrics)
	sMux.HandleFunc("POST /api/users", cfg.handlerCreateUser)
	sMux.HandleFunc("POST /api/chirps", cfg.handlerCreateChirp)
	sMux.HandleFunc("POST /api/login", cfg.handlerLogin)
	sMux.HandleFunc("POST /api/refresh", cfg.handlerRefresh)
	sMux.HandleFunc("POST /api/revoke", cfg.handlerRevoke)
	sMux.HandleFunc("POST /api/polka/webhooks", cfg.handlerUpdgradeUser)
	sMux.HandleFunc("PUT /api/users", cfg.handlerUpdateUser)
	sMux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handleDeleteChirp)
	log.Fatal(newServer.ListenAndServe())
}
