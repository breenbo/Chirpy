package main

import "net/http"

func main() {
	// create a server
	serveMux := http.NewServeMux()
	// serve index.html from root directory to /, remove the prefix from the url so it serves files from root directory
	serveMux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	// serve the healthz endpoint
	serveMux.HandleFunc("/healthz", readiness)
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
