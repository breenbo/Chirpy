package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/breenbo/chirpy/internal/database"
	"github.com/breenbo/chirpy/internal/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	err = godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// get the secret to use for JWT
	secret := os.Getenv("SECRET")

	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
		secret:         secret,
	}

	initServer(apiCfg)
}

// count number of hits to fileserver
// get queries to work with db
type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	secret         string
}

func initDB() (*sql.DB, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initServer(apiCfg *apiConfig) {
	userHandler := handlers.NewUserHandler(apiCfg.dbQueries, apiCfg.secret)
	chirpHandler := handlers.NewChirpHandler(apiCfg.dbQueries)

	// create a server
	serveMux := http.NewServeMux()
	// serve index.html from root directory to /, remove the prefix from the url so it serves files from root directory
	serveMux.Handle("/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// serve the healthz endpoint
	serveMux.HandleFunc("GET /admin/healthz", readiness)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.getMetricsHandler)
	// users
	serveMux.HandleFunc("POST /api/login", userHandler.Login)
	serveMux.HandleFunc("POST /api/users", userHandler.CreateUser)
	serveMux.HandleFunc("POST /admin/reset", userHandler.ResetUsers)
	// chirps
	serveMux.HandleFunc("POST /api/chirps", chirpHandler.CreateChirps)
	serveMux.HandleFunc("GET /api/chirps", chirpHandler.GetChirps)
	serveMux.HandleFunc("GET /api/chirps/{id}", chirpHandler.GetOneChirp)
	//
	// setup the server
	//
	server := http.Server{
		Addr:    ":8080",
		Handler: serveMux,
	}
	// start the server
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		log.Fatal(err)
	}
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

func (cfg *apiConfig) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html;charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := fmt.Fprintf(w, `
		<html>
		  <body>
		    <h1>Welcome, Chirpy Admin</h1>
		    <p>Chirpy has been visited %d times!</p>
		  </body>
		</html>
	`, cfg.fileserverHits.Load())
	if err != nil {
		log.Fatal(err)
	}
}
