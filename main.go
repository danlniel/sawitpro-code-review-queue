// main.go
package main

import (
	"fmt"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, Railway! Your request is: %s", r.URL.Path[1:])
}

func main() {
	http.HandleFunc("/", handler)
	port := "8080"
	fmt.Printf("Starting server on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}
