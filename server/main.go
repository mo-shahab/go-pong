package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

type paddleData struct {
	movementSum int
	players     int
	position    int
}

type ball struct {
	x, y   float64
	dx, dy float64
}

type canvas struct {
  width float64
  height float64
}

type Client struct {
	conn      *websocket.Conn
	sendQueue chan interface{}
	team      string
}

type webSocketHandler struct {
	upgrader        websocket.Upgrader
	leftPaddleData  paddleData
	rightPaddleData paddleData
	ballVar         ball
  canvasVar canvas
	mu              sync.Mutex
	connections     map[*websocket.Conn]*Client
}

type paddlePositions struct {
	leftPaddle  int
	rightPaddle int
}

var globalPaddlePositions = &paddlePositions{leftPaddle: 0, rightPaddle: 0}

func (wsh *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error %s when connecting to the socket", err)
		return
	}

	client := &Client{
		conn:      conn,
		sendQueue: make(chan interface{}, 100), // Increased buffer size
	}

  // a message queue, that sends the data to the client
	go func() {
		for msg := range client.sendQueue {
			err := client.conn.WriteJSON(msg)
			if err != nil {
				log.Println("Write error:", err)
				client.conn.Close()
				close(client.sendQueue)

				wsh.mu.Lock()
				delete(wsh.connections, conn)
				wsh.mu.Unlock()
				return
			}
		}
	}()

	wsh.mu.Lock()

	if len(wsh.connections) == 0 {
		globalPaddlePositions.leftPaddle = 0
		globalPaddlePositions.rightPaddle = 0
		wsh.leftPaddleData.position = 0
		wsh.rightPaddleData.position = 0
	}

	if len(wsh.connections)%2 == 0 {
		client.team = "left"
		wsh.leftPaddleData.players++
	} else {
		client.team = "right"
		wsh.rightPaddleData.players++
	}

	wsh.connections[conn] = client
	wsh.mu.Unlock()

	initialGameState := map[string]interface{}{
		"leftPaddleData":  globalPaddlePositions.leftPaddle,
		"rightPaddleData": globalPaddlePositions.rightPaddle,
		"yourTeam":        client.team,
	}

	client.sendQueue <- initialGameState

	// Handle incoming messages
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from the client: %s", err)
			wsh.disconnectPlayer(conn)
			return
		}

		log.Printf("Message received: %s", p)

		var msg struct {
			Type      string `json:"type"`
			Direction string `json:"direction"`
			Paddle    string `json:"paddle"`
      Width float64 `json:width,omitempty`
      Height float64 `json:height,omitempty`
		}

		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Printf("Error unmarshalling message: %s", err)
			// Send JSON error response
			client.sendQueue <- map[string]string{"error": "Invalid message format"}
			continue
		}

    if msg.Type == "init" {
      wsh.mu.Lock()
      if len(wsh.connections) == 0 {
        wsh.ballVar = ball {
          x: 300,
          y: 400, 
          dx: 2,
          dy: 2,
        }

        wsh.canvasVar.width = msg.Width
        wsh.canvasVar.height = msg.Height
      }
      wsh.mu.Unlock()
    }

    wsh.mu.Lock()
    if len(wsh.connections) == 1 {
        go wsh.startBallUpdates()
    }
    wsh.mu.Unlock()

		// Validate message
    /*
		valid := true
		if msg.Type != "move" || msg.Type != "init" {
			log.Println("Invalid type:", msg.Type)
			valid = false
		}

    if msg.Type == "move" {
      if msg.Direction != "up" && msg.Direction != "down" {
        log.Println("Invalid direction:", msg.Direction)
        valid = false
      }
    }

		if !valid {
			log.Println("Invalid message received")
			client.sendQueue <- map[string]string{"error": "Invalid message parameters"}
			continue
		}

		log.Printf("Valid message received: %+v\n", msg)
    */
		client.sendQueue <- map[string]string{"status": "Message processed"}

		var movement int
		if msg.Direction == "up" {
			movement = -10
		} else if msg.Direction == "down" {
			movement = 10
		}

		wsh.mu.Lock()
		if client.team == "left" {
			wsh.leftPaddleData.movementSum += movement
			if wsh.leftPaddleData.players > 0 {
				wsh.leftPaddleData.position = wsh.leftPaddleData.movementSum / wsh.leftPaddleData.players
				wsh.rightPaddleData.position = 0
				wsh.leftPaddleData.movementSum = 0
			} else {
				wsh.leftPaddleData.position = 0
				wsh.leftPaddleData.movementSum = 0
			}
			globalPaddlePositions.leftPaddle += movement
		} else {
			wsh.rightPaddleData.movementSum += movement
			if wsh.rightPaddleData.players > 0 {
				wsh.rightPaddleData.position = wsh.rightPaddleData.movementSum / wsh.rightPaddleData.players
				wsh.leftPaddleData.position = 0
				wsh.rightPaddleData.movementSum = 0
			} else {
				wsh.rightPaddleData.position = 0
				wsh.rightPaddleData.movementSum = 0
			}
			globalPaddlePositions.rightPaddle += movement
		}
		wsh.mu.Unlock()

		wsh.broadcastPaddlePositions()

	}
}

func (wsh *webSocketHandler) startBallUpdates() {
	log.Println("startBallUpdates started") // Debugging log

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		wsh.mu.Lock()
		if len(wsh.connections) == 0 {
			log.Println("No active connections, skipping update") // Debugging log
			wsh.mu.Unlock()
			continue
		}
		wsh.mu.Unlock()

		log.Println("Updating ball position") // Debugging log
		wsh.updateBallPosition()

		wsh.mu.Lock()
		message := map[string]interface{}{
			"ball": map[string]float64{
				"x": wsh.ballVar.x,
				"y": wsh.ballVar.y,
			},
		}
		wsh.mu.Unlock()

		log.Println("Broadcasting message:", message) // Debugging log
		wsh.broadcastToAll(message)
	}
}

func (wsh *webSocketHandler) disconnectPlayer(conn *websocket.Conn) {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	client, exists := wsh.connections[conn]
	if !exists {
		return
	}

	if client.team == "left" {
		wsh.leftPaddleData.players--
	} else {
		wsh.rightPaddleData.players--
	}

	close(client.sendQueue)
	delete(wsh.connections, conn)
}

func (wsh *webSocketHandler) broadcastPaddlePositions() {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	// Prepare game state with current paddle positions
	gameState := map[string]int{
		"leftPaddleData":  wsh.leftPaddleData.position,
		"rightPaddleData": wsh.rightPaddleData.position,
	}

	for _, client := range wsh.connections {
		select {
		case client.sendQueue <- gameState:
		default:
			log.Println("Dropping message, send queue full for client")
		}
	}
}

func (wsh *webSocketHandler) broadcastToAll(message interface{}) {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()
	log.Println("Broadcasting message:", message) 

	for conn, client := range wsh.connections {
		select {
		case client.sendQueue <- message:
			log.Println("Message sent to client:", conn.RemoteAddr()) 
		default:
			log.Println("Dropping message, send queue full for client", conn.RemoteAddr())
		}
	}
}

func (wsh *webSocketHandler) updateBallPosition() {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

  wsh.ballVar.x += wsh.ballVar.dx
  log.Println(wsh.ballVar.x)
  log.Println(wsh.ballVar.y)

  wsh.ballVar.y += wsh.ballVar.dy

	maxWidth := wsh.canvasVar.width
	maxHeight := wsh.canvasVar.height

	if wsh.ballVar.x <= 0 || wsh.ballVar.x >= maxWidth {
		wsh.ballVar.dx *= -1
	}

	if wsh.ballVar.y <= 0 || wsh.ballVar.y >= maxHeight {
		wsh.ballVar.dy *= -1
	}
}

func main() {
	wsh := &webSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[*websocket.Conn]*Client),
	}

	fs := http.FileServer(http.Dir("../client/"))

	http.Handle("/", fs)
	http.Handle("/ws", wsh)
	log.Println("Server starting at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
