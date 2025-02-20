package main

import (
	"log"
	"net/http"
  "github.com/mo-shahab/go-pong/wsserver"
)

func main() {
	wsh := wsserver.NewWebSocketHandler();
	fs := http.FileServer(http.Dir("../client/"))

	http.Handle("/", fs)
	http.Handle("/ws", wsh)
	log.Println("Server starting at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
