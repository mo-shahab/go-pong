package main

import (
  "log"
  "net/http"
  "encoding/json"
  "sync"
  "github.com/gorilla/websocket"
)

type paddleData struct {
  movementSum int
  players int 
  position int
}

type webSocketHandler struct {
  upgrader websocket.Upgrader
  leftPaddleData paddleData
  rightPaddleData paddleData
  mu sync.Mutex
  connections map[*websocket.Conn]string
}

func (wsh webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  conn, err := wsh.upgrader.Upgrade(w, r, nil)
  if err != nil {
    log.Println("Error %s when connecting to the socket", err)
    return 
  }

  defer conn.Close()

  // assign connection (player) to a team (paddle)
  wsh.mu.Lock()
  var team string
  if len(wsh.connections) % 2 == 0 {
    team = "left"
    wsh.leftPaddleData.players++
  } else {
    team = "right"
    wsh.rightPaddleData.players++
  }

  wsh.connections[conn] = team
  wsh.mu.Unlock()

  for {
    _, p, err := conn.ReadMessage()
    if err != nil {
      log.Printf("error reading message from the client %s", err)
      wsh.disconnectPlayer(conn, team)
      return 
    }

    log.Printf("Message received: %s", p)

    var msg struct {
      Type      string `json:"type"`
      Direction string `json:"direction"`
      Paddle    string `json:"paddle"`
    }

    err = json.Unmarshal(p, &msg)
    if err != nil {
      log.Printf("error unmarshalling message %s", err)
      conn.WriteMessage(websocket.TextMessage, []byte("Invalid message received"))
      continue
    }

    log.Printf("Parsed Message - Type: %s, Direction: %s, Paddle: %s", msg.Type, msg.Direction, msg.Paddle)

    valid := true
		if msg.Type != "move" {
			log.Println("Invalid type:", msg.Type)
			valid = false
		}
		if msg.Direction != "up" && msg.Direction != "down" {
			log.Println("Invalid direction:", msg.Direction)
			valid = false
		}
		if msg.Paddle != "left" && msg.Paddle != "right" {
			log.Println("Invalid paddle:", msg.Paddle)
			valid = false
		}

		if !valid {
			log.Println("Invalid message received")
			conn.WriteMessage(websocket.TextMessage, []byte("Invalid message format"))
			continue
		}

		// Handle valid message
		log.Printf("Valid message received: %+v\n", msg)
		conn.WriteMessage(websocket.TextMessage, []byte("Message processed"))

    var movement int
    if(msg.Direction == "up"){
      movement = -10
    } else if (msg.Direction  == "down"){
      movement = 10
    } else {
      log.Println("Invalid message received")
      continue  
    }

    // update the paddle position
    wsh.mu.Lock()
    if team == "left"{
      wsh.leftPaddleData.movementSum += movement
      wsh.leftPaddleData.position = wsh.leftPaddleData.movementSum / wsh.leftPaddleData.players
      wsh.leftPaddleData.movementSum = 0
    } else {
      wsh.rightPaddleData.movementSum += movement
      wsh.rightPaddleData.position = wsh.rightPaddleData.movementSum / wsh.rightPaddleData.players
      wsh.rightPaddleData.movementSum = 0
    }
    wsh.mu.Unlock()

    wsh.broadcastPaddlePositions()
  }
}

func (wsh *webSocketHandler) disconnectPlayer (conn *websocket.Conn, team string){
  wsh.mu.Lock() 
  defer wsh.mu.Unlock()

  delete(wsh.connections, conn)
  if team == "left" {
    wsh.leftPaddleData.players--
  } else {
    wsh.rightPaddleData.players--
  }
}

func (wsh *webSocketHandler) broadcastPaddlePositions (){
  wsh.mu.Lock() 
  defer wsh.mu.Unlock()
  
  // prepare game state
  gameState := map[string]int{
    "leftPaddleData": wsh.leftPaddleData.position,
    "rightPaddleData": wsh.rightPaddleData.position,
  }

  for conn, _ := range wsh.connections {
    err := conn.WriteJSON(gameState)
    if err != nil {
      log.Printf("error writing to the client %s", err)
      wsh.disconnectPlayer(conn, wsh.connections[conn])
    }
  }
}

func main(){
  wsh := &webSocketHandler{
    upgrader: websocket.Upgrader{
      CheckOrigin: func(r *http.Request) bool { return true },
    },
    // a memory allocation for all the connections
    connections: make(map[*websocket.Conn]string),
  }

  fs := http.FileServer(http.Dir("../client/"))

  http.Handle("/", fs)
  http.Handle("/ws", wsh)
  log.Println("Server starting")
  log.Fatal(http.ListenAndServe(":8080", nil))
  
}
