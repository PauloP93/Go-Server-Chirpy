package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PauloP93/chirpy/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
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
	html := fmt.Sprintf("<html> <body> <h1>Welcome, Chirpy Admin</h1> <p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())
	wr.Header().Add("Content-Type", "text/html")
	wr.Write([]byte(html))
}

//helpers

func respondWithError(wr http.ResponseWriter, code int, msg string) {
	respFail, errMarsh := json.Marshal(msg)
	wr.WriteHeader(code)
	if errMarsh != nil {
		log.Printf("Error marshalling JSON: %s", errMarsh)
		return
	}

	wr.Write(respFail)
}

func respondWithJSON(wr http.ResponseWriter, code int, payload interface{}) {

	wr.WriteHeader(code)
	respSuccess, errMarsh := json.Marshal(payload)
	if errMarsh != nil {
		log.Printf("Error marshalling JSON: %s", errMarsh)
		wr.WriteHeader(500)
		return
	}
	wr.Write(respSuccess)
}

func processChirpyProfaneWords(body string) string {
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}

	bodyArr := strings.Split(body, " ")

	for idx, bodyWrd := range bodyArr {
		lowerBodyWrd := strings.ToLower(bodyWrd)
		for _, profWrd := range profaneWords {
			if strings.Contains(lowerBodyWrd, profWrd) {

				bodyArr[idx] = "****"

			}
		}
	}

	return strings.Join(bodyArr, " ")
}

// MAIN
func main() {
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		log.Fatalf("Error occurred while creating connection to DB: %s", err.Error())
	}

	dbQueries := database.New(db)

	apiCfg := apiConfig{db: dbQueries}
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

	serverMux.HandleFunc("POST /api/users", func(wr http.ResponseWriter, req *http.Request) {
		type User struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Email     string    `json:"email"`
		}

		decoder := json.NewDecoder(req.Body)
		usrReq := User{}

		errDecode := decoder.Decode(&usrReq)
		if errDecode != nil {
			log.Fatalf("Error occurred while decoding: %s", errDecode.Error())
			respondWithError(wr, 500, "error: Something went wrong")
			return
		}

		if usrReq.Email == "" {
			respondWithError(wr, 400, "error: Email is empty")
			return
		}

		user, err := apiCfg.db.CreateUser(req.Context(), usrReq.Email)

		if err != nil {
			respondWithError(wr, 500, "error: Email is empty")
			return
		}

		respondWithJSON(wr, 201, user)
	})

	serverMux.HandleFunc("POST /api/validate_chirp", func(wr http.ResponseWriter, req *http.Request) {

		type reqBody struct {
			Body string `json:"body"`
		}

		decoder := json.NewDecoder(req.Body)
		body := reqBody{}

		err := decoder.Decode(&body)
		wr.Header().Add("Content-Type", "text/json")

		if err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(wr, 500, "error: Something went wrong")
			return
		}

		if len(body.Body) >= 140 {
			respondWithError(wr, 400, "error: Chirp is too long")
			return
		}

		bodyProc := processChirpyProfaneWords(body.Body)
		respondWithJSON(wr, 200, map[string]string{"cleaned_body": bodyProc})
	})

	serverMux.HandleFunc("GET /api/healthz", func(wr http.ResponseWriter, req *http.Request) {
		wr.WriteHeader(200)
		wr.Header().Add("Content-Type", "text/plain; charset=utf-8")
		wr.Write([]byte("OK"))
	})

	serverMux.HandleFunc("GET /admin/metrics", apiCfg.countRequests)
	serverMux.HandleFunc("POST /admin/reset", apiCfg.resetCount)

	server.ListenAndServe()
}
