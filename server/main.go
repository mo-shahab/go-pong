package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type paddleData struct {
	movementSum float64
  velocity    float64
	players     int
	position    float64
}

// note: this is different from the paddleData, the below struct is used for
// the rendering of the paddle, while the paddleData is used, for the socket
// and the server interaction

type paddle struct {
	height float64
	width  float64
}

type ball struct {
	x, y    float64
	dx, dy  float64
	radius  float64
	visible bool
}

type canvas struct {
	width  float64
	height float64
}

type Client struct {
	conn      *websocket.Conn
	sendQueue chan interface{}
	team      string
	id        string
}

type webSocketHandler struct {
	upgrader        websocket.Upgrader
	leftPaddleData  paddleData
	rightPaddleData paddleData
	ballVar         ball
	canvasVar       canvas
	paddleVar       paddle
	mu              sync.Mutex
	connections     map[string]*Client
	connToId        map[*websocket.Conn]string
	ballRunning     bool
	ballVisible     bool
}

type paddlePositions struct {
	leftPaddle  float64
	rightPaddle float64
}

// ball constants
const (
	initialBallDx = 20
	initialBallDy = 20
	ballRadius    = 8
)

// paddle constants
const (
  maxSpeed = 10.0
  acceleration = 2.0
  friction = 0.9
)

var globalPaddlePositions = &paddlePositions{}

func (wsh *webSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error %s when connecting to the socket", err)
		return
	}

	clientId := conn.RemoteAddr().String() + "_" + time.Now().String()

	client := &Client{
		conn:      conn,
		sendQueue: make(chan interface{}, 100), // Increased buffer size
		id:        clientId,
	}

  globalPaddlePositions.leftPaddle = (wsh.canvasVar.height / 2) - (wsh.paddleVar.height / 2) 
  globalPaddlePositions.rightPaddle = (wsh.canvasVar.height / 2) - (wsh.paddleVar.height / 2)

	// a message queue, that sends the data to the client
	go func() {
		for msg := range client.sendQueue {
			err := client.conn.WriteJSON(msg)
			if err != nil {
				log.Println("Write error:", err)
				client.conn.Close()
				wsh.disconnectPlayer(conn)
				return
			}
		}
	}()

	wsh.mu.Lock()

  if len(wsh.connections) == 0 {
    globalPaddlePositions.leftPaddle = (wsh.canvasVar.height / 2) - (wsh.paddleVar.height / 2)
    globalPaddlePositions.rightPaddle = (wsh.canvasVar.height / 2) - (wsh.paddleVar.height / 2)
    wsh.leftPaddleData.position = globalPaddlePositions.leftPaddle
    wsh.rightPaddleData.position = globalPaddlePositions.rightPaddle
    wsh.ballRunning = false
    wsh.ballVisible = false
  }

	if len(wsh.connections)%2 == 0 {
		client.team = "left"
		wsh.leftPaddleData.players++
	} else {
		client.team = "right"
		wsh.rightPaddleData.players++
	}

	wsh.connections[clientId] = client
	wsh.connToId[conn] = clientId

	log.Println("Total number of connections: ", len(wsh.connections))
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
			Type         string  `json:"type"`
			Direction    string  `json:"direction"`
			Paddle       string  `json:"paddle"`
			Width        float64 `json:width,omitempty`
			Height       float64 `json:height,omitempty`
			PaddleHeight float64 `json:paddleHeight,omitempty`
			PaddleWidth  float64 `json:paddleWidth,omitempty`
		}

		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Printf("Error unmarshalling message: %s", err)
			// Send JSON error response
			client.sendQueue <- map[string]string{"error": "Invalid message format"}
			continue
		}

		if msg.Width > 0 && msg.Height > 0 {
			wsh.mu.Lock()
			wsh.ballVar = ball{
				x:       msg.Width / 2,
				y:       msg.Height / 2,
				dx:      10,
				dy:      10,
				radius:  ballRadius,
				visible: true,
			}

			wsh.canvasVar.width = msg.Width
			wsh.paddleVar.width = msg.PaddleWidth
			wsh.paddleVar.height = msg.PaddleHeight
			wsh.canvasVar.height = msg.Height

			if !wsh.ballRunning && len(wsh.connections) > 1 {
				wsh.ballRunning = true
				go wsh.startBallUpdates()
			}
			wsh.mu.Unlock()
		}

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

    var movement float64
    		if msg.Direction == "up" {
			movement = -30
		} else if msg.Direction == "down" {
			movement = 30
		}

    wsh.mu.Lock()
    if client.team == "left" {
      newLeftPaddlePos := globalPaddlePositions.leftPaddle + movement

      if newLeftPaddlePos >= 0 && newLeftPaddlePos + wsh.paddleVar.height <= wsh.canvasVar.height {
        globalPaddlePositions.leftPaddle = newLeftPaddlePos
      }

      wsh.leftPaddleData.movementSum += movement
      if wsh.leftPaddleData.players > 0 {
        wsh.leftPaddleData.position = wsh.leftPaddleData.movementSum / float64(wsh.leftPaddleData.players)
        wsh.rightPaddleData.position = 0
        wsh.leftPaddleData.movementSum = 0
      } else {
        wsh.leftPaddleData.position = 0
        wsh.leftPaddleData.movementSum = 0
      }
    } else {
      newRightPaddlePos := globalPaddlePositions.rightPaddle + movement

      if newRightPaddlePos >= 0 && newRightPaddlePos + wsh.paddleVar.height <= wsh.canvasVar.height {
        globalPaddlePositions.rightPaddle = newRightPaddlePos
      }

      wsh.rightPaddleData.movementSum += movement
      if wsh.rightPaddleData.players > 0 {
        wsh.rightPaddleData.position = wsh.rightPaddleData.movementSum / float64(wsh.rightPaddleData.players)
        wsh.leftPaddleData.position = 0
        wsh.rightPaddleData.movementSum = 0
      } else {
        wsh.rightPaddleData.position = 0
        wsh.rightPaddleData.movementSum = 0
      }
    }
    wsh.mu.Unlock()
		wsh.broadcastPaddlePositions()
  }
}

func (wsh *webSocketHandler) startBallUpdates() {

	ticker := time.NewTicker(32 * time.Millisecond)
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

		wsh.updateBallPosition()

		wsh.mu.Lock()
		message := map[string]interface{}{
			"ball": map[string]float64{
				"x":      wsh.ballVar.x,
				"y":      wsh.ballVar.y,
				"radius": wsh.ballVar.radius,
			},
		}
		wsh.mu.Unlock()

		wsh.broadcastToAll(message)
	}
}

func (wsh *webSocketHandler) disconnectPlayer(conn *websocket.Conn) {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	clientId, exists := wsh.connToId[conn]
	if !exists {
		return
	}

	client, exists := wsh.connections[clientId]

	if client.team == "left" {
		wsh.leftPaddleData.players--
	} else {
		wsh.rightPaddleData.players--
	}

	close(client.sendQueue)
	delete(wsh.connections, clientId)
	delete(wsh.connToId, conn)

	conn.Close()
}

func (wsh *webSocketHandler) broadcastPaddlePositions() {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	// Prepare game state with current paddle positions
	gameState := map[string]float64{
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

	for _, client := range wsh.connections {
		select {
		case client.sendQueue <- message:
			// log.Println("Message sent to client:", conn.RemoteAddr())
		default:
			log.Println("Dropping message, send queue full for client")
		}
	}
}

func (wsh *webSocketHandler) updateBallPosition() {
	wsh.mu.Lock()
	defer wsh.mu.Unlock()

	// update ball position
	wsh.ballVar.x += wsh.ballVar.dx
	wsh.ballVar.y += wsh.ballVar.dy

	maxWidth := wsh.canvasVar.width
	maxHeight := wsh.canvasVar.height
	ballRadius := wsh.ballVar.radius

	// wall collision (top & bottom)
	if wsh.ballVar.y-ballRadius <= 0 || wsh.ballVar.y+ballRadius >= maxHeight {
		wsh.ballVar.dy *= -1
	}

	// wall collision (left & right)
	if wsh.ballVar.x-ballRadius <= 0 || wsh.ballVar.x+ballRadius >= maxWidth {
		wsh.ballVar.dx *= -1
	}

	// paddle collision logic
	wsh.checkPaddleCollision()
}

func(wsh *webSocketHandler) updatePaddlePositions(client *Client, direction string) {
  wsh.mu.Lock()
  defer wsh.mu.Unlock()

  var paddle *paddleData
  var globalPosition *float64

  if client.team == "left" {
    paddle = &wsh.leftPaddleData
    globalPosition = &globalPaddlePositions.leftPaddle
  } else {
    paddle = &wsh.leftPaddleData
    globalPosition = &globalPaddlePositions.leftPaddle
  }

  if direction == "up" {
    paddle.velocity -= acceleration
  } else if direction == "down" {
    paddle.velocity += acceleration
  } else {
    paddle.velocity *= friction
  }

  if paddle.velocity > maxSpeed {
    paddle.velocity = maxSpeed
  } else if paddle.velocity < -maxSpeed {
    paddle.velocity = -maxSpeed
  }
  
  newPosition := *globalPosition + paddle.velocity

  if newPosition < 0 {
    newPosition = 0
    paddle.velocity = 0
  } else if newPosition+wsh.paddleVar.height > wsh.canvasVar.height {
    newPosition = wsh.canvasVar.height - wsh.paddleVar.height
    paddle.velocity = 0
  }

  paddle.movementSum += paddle.velocity

  if paddle.players >  0 {
    paddle.position = paddle.movementSum / float64(paddle.players)
    paddle.movementSum = 0
  } else {
    paddle.position = 0
    paddle.movementSum = 0
  }

  wsh.broadcastPaddlePositions()
}

// broken and stuff how to fix this thi
func (wsh *webSocketHandler) checkPaddleCollision() {
	ballRadius := wsh.ballVar.radius

	leftPaddleRight := wsh.paddleVar.width
	leftPaddleTop := float64(globalPaddlePositions.leftPaddle)
	leftPaddleBottom := leftPaddleTop + float64(wsh.paddleVar.height)


	rightPaddleLeft := wsh.canvasVar.width - wsh.paddleVar.width
	rightPaddleTop := float64(globalPaddlePositions.rightPaddle)
	rightPaddleBottom := rightPaddleTop + float64(wsh.paddleVar.height)

	ballSpeed := math.Hypot(wsh.ballVar.dx, wsh.ballVar.dy)
	maxBounceAngle := math.Pi / 3 // 60 degrees max

	if wsh.ballVar.x-ballRadius <= leftPaddleRight &&
		wsh.ballVar.y >= leftPaddleTop &&
		wsh.ballVar.y <= leftPaddleBottom {
      log.Println("collision with the left paddle detected")
   
      relativePosition := (wsh.ballVar.y - (leftPaddleTop + float64(wsh.paddleVar.height)/2)) / (float64(wsh.paddleVar.height) / 2)
      bounceAngle := relativePosition * maxBounceAngle
      wsh.ballVar.dx = math.Abs(ballSpeed * math.Cos(bounceAngle))
      wsh.ballVar.dy = ballSpeed * math.Sin(bounceAngle)
      wsh.ballVar.dy += randomVariation()
      wsh.ballVar.x = leftPaddleRight + ballRadius
    }

	if wsh.ballVar.x+ballRadius >= rightPaddleLeft &&
		wsh.ballVar.y >= rightPaddleTop &&
		wsh.ballVar.y <= rightPaddleBottom {
      log.Println("collision with the right paddle detected")
      relativePosition := (wsh.ballVar.y - (rightPaddleTop + float64(wsh.paddleVar.height)/2)) / (float64(wsh.paddleVar.height) / 2)
      bounceAngle := relativePosition * maxBounceAngle
      wsh.ballVar.dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
      wsh.ballVar.dy = ballSpeed * math.Sin(bounceAngle)
      wsh.ballVar.dy += randomVariation()
      wsh.ballVar.x = rightPaddleLeft - ballRadius
    }
}

func randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
}

func (wsh *webSocketHandler) resetBall() {
	wsh.ballVar.x = wsh.canvasVar.width / 2
	wsh.ballVar.y = wsh.canvasVar.height / 2

	// reverse the direction randomly for variation
	wsh.ballVar.dx = -wsh.ballVar.dx
}

func main() {
	wsh := &webSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[string]*Client),
		connToId:    make(map[*websocket.Conn]string),
	}

	fs := http.FileServer(http.Dir("../client/"))

	http.Handle("/", fs)
	http.Handle("/ws", wsh)
	log.Println("Server starting at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
