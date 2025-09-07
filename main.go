package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/breenbo/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
	}

	initServer(apiCfg)

}

// count number of hits to fileserver
// get queries to work with db
type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func initDB() (*sql.DB, error) {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initServer(apiCfg *apiConfig) {
	// create a server
	serveMux := http.NewServeMux()
	// serve index.html from root directory to /, remove the prefix from the url so it serves files from root directory
	serveMux.Handle("/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// serve the healthz endpoint
	serveMux.HandleFunc("GET /admin/healthz", readiness)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.getMetricsHandler)
	// serveMux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.resetUsersHandler)
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
	serveMux.HandleFunc("POST /api/users", apiCfg.createUserHandler)
	//
	// setup the server
	//
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	// start the server
	server.ListenAndServe()
}

func readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// called on each request, but why???
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// increment the counter
		cfg.fileserverHits.Add(1)
		// pass the request to the file server
		next.ServeHTTP(w, r)
	})
}

//	func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
//		cfg.fileserverHits.Store(0)
//	}
func (cfg *apiConfig) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `
		<html>
		  <body>
		    <h1>Welcome, Chirpy Admin</h1>
		    <p>Chirpy has been visited %d times!</p>
		  </body>
		</html>
	`, cfg.fileserverHits.Load())
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type reqBody struct {
		Body string `json:"body"`
	}
	type okRes struct {
		Cleaned_body string `json:"cleaned_body"`
	}
	type errorRes struct {
		Error string `json:"error"`
	}
	// get the json body
	decoder := json.NewDecoder(r.Body)
	req_body := reqBody{}
	err := decoder.Decode(&req_body)
	if err != nil {
		returnParseError(w, "error parsing request body")
		return
	}

	// return error if body is too long
	if len(req_body.Body) > 140 {
		w.Header().Set("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(400)
		resBody := errorRes{
			Error: "Chirp is too long",
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			w.Write([]byte("error parsing json"))
		} else {
			w.Write(data)
		}
		return
	}

	// return valid response
	w.Header().Set("Content-type", " application/json;charset=utf-8")
	w.WriteHeader(200)
	response := okRes{
		Cleaned_body: replaceBadWords(req_body.Body),
	}
	data, err := json.Marshal(&response)
	if err != nil {
		w.Write([]byte("error parsing json"))
	} else {
		w.Write(data)
	}
}

func replaceBadWords(body string) string {
	badwords := []string{"kerfuffle", "sharbert", "fornax"}
	strArray := strings.Split(body, " ")

	for i, word := range strArray {
		if slices.Contains(badwords, strings.ToLower(word)) {
			strArray[i] = "****"
		}
	}

	return strings.Join(strArray, " ")
}

func returnParseError(w http.ResponseWriter, msg string) {
	type errorRes struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-type", "application/json;charset=utf-8")
	w.WriteHeader(500)
	resBody := errorRes{
		Error: msg,
	}
	data, err := json.Marshal(resBody)
	if err != nil {
		w.Write([]byte("error parsing json"))
	} else {
		w.Write(data)
	}
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type User struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}
	type Request struct {
		Email string `json:"email"`
	}
	type errorRes struct {
		Error string `json:"error"`
	}

	// get the body of the request
	decoder := json.NewDecoder(r.Body)
	req_body := Request{}
	err := decoder.Decode(&req_body)
	if err != nil {
		msg := fmt.Sprintf("Error parsing request body: %v", err)
		returnParseError(w, msg)
		return
	}

	// create user in db
	res, err := cfg.dbQueries.CreateUser(r.Context(), req_body.Email)
	if err != nil {
		msg := fmt.Sprintf("Error creating user: %v", err)
		returnParseError(w, msg)
		return
	}

	// return the user after being created
	w.Header().Set("Content-type", "application/json;charset=utf-8")
	w.WriteHeader(201)
	response := User{
		ID:        res.ID,
		CreatedAt: res.CreatedAt,
		UpdatedAt: res.UpdatedAt,
		Email:     res.Email,
	}
	data, err := json.Marshal(response)
	if err != nil {
		w.Write([]byte("Error parsing response json"))
	} else {
		w.Write(data)
	}
}

func (cfg *apiConfig) resetUsersHandler(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")
	if platform != "dev" {
		w.WriteHeader(403)
		return
	}

	err := cfg.dbQueries.ResetUsers(r.Context())
	if err != nil {
		w.Header().Add("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(500)
	}

	w.WriteHeader(200)
}
