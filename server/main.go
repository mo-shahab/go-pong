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
    messageType, p, err := conn.ReadMessage()
    if err != nil {
      log.Printf("error reading message from the client %s", err)
      return 
    }
    log.Printf("recieved message from teh client : %s", string(p))
    if err:= conn.WriteMessage(messageType, p); err != nil {
      log.Printf("error writing message to the client %s", err)
      return
    }
  }


  defer conn.Close()
}

func main(){
  wsh := webSocketHandler{
    upgrader: websocket.Upgrader{},
  }

  fs := http.FileServer(http.Dir("../client/"))

  http.Handle("/", fs)
  http.Handle("/ws", wsh)
  log.Println("Server starting")
  log.Fatal(http.ListenAndServe(":8080", nil))
  
}
