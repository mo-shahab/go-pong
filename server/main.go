package main

import (
	"github.com/mo-shahab/go-pong/wsserver"
	"log"
	"net/http"
)

func main() {
	wsh := wsserver.NewWebSocketHandler()

	// no need to server files on http now
	// fs := http.FileServer(http.Dir("../client/"))
	// http.Handle("/", fs)

	http.Handle("/ws", wsh)
	log.Println("Server starting at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
