package main

import (
  "log"
  "net/http"
  "github.com/gorilla/websocket"
)

type webSocketHandler struct {
  upgrader websocket.Upgrader
}

func (wsh webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  conn, err := wsh.upgrader.Upgrade(w, r, nil)
  if err != nil {
    log.Println("Error %s when connecting to the socket", err)
    return 
  }

  for {
    mt, message, err := conn.ReadMessage()
  
    if err != nil {
      log.Printf("Error %s when reading the message from client", err)
      return 
    }


  }

  defer conn.Close()
}

func main(){
  wsh := webSocketHandler{
    upgrader: websocket.Upgrader{},
  }

  http.Handle("/ws", wsh)
  log.Println("Server starting")
  log.Fatal(http.ListenAndServe(":8080", nil))
  
}
