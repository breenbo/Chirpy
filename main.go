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

	"github.com/breenbo/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	_, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	// dbQueries := database.New(db)

	// create a server
	serveMux := http.NewServeMux()
	// serve index.html from root directory to /, remove the prefix from the url so it serves files from root directory
	apiCfg := &apiConfig{
		fileserverHits: atomic.Int32{},
	}
	serveMux.Handle("/app/",
		apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// serve the healthz endpoint
	serveMux.HandleFunc("GET /admin/healthz", readiness)
	serveMux.HandleFunc("GET /admin/metrics", apiCfg.getMetricsHandler)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.resetMetricsHandler)
	serveMux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
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

// count number of hits to fileserver
type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
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
func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
}
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
		w.Header().Set("Content-type", "application/json;charset=utf-8")
		w.WriteHeader(500)
		resBody := errorRes{
			Error: "error decoding request body",
		}
		data, err := json.Marshal(resBody)
		if err != nil {
			w.Write([]byte("error parsing json"))
		} else {
			w.Write(data)
		}
		return
	}

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
