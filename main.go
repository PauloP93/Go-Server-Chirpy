package main

import (
	"net/http"
)

func main() {

	serverMux := http.NewServeMux()
	server := http.Server{
		Handler: serverMux,
		Addr:    ":8080",
	}

	serverMux.HandleFunc("/healthz", func(wr http.ResponseWriter, req *http.Request) {

		wr.WriteHeader(200)
		wr.Header().Add("Content-Type", "text/plain; charset=utf-8")
		wr.Write([]byte("OK"))

	})

	fs := http.FileServer(http.Dir("."))
	serverMux.Handle("/app/", http.StripPrefix("/app/", fs))
	serverMux.Handle("/assets", http.StripPrefix("/app/", fs))

	server.ListenAndServe()

}
