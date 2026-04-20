package main

import (
	"log"
	"net/http"
	"os"
)

func handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler()))
}
