package wsserver

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/mo-shahab/go-pong/ball"
	"github.com/mo-shahab/go-pong/canvas"
	"github.com/mo-shahab/go-pong/paddle"
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

type Client struct {
	conn      *websocket.Conn
	sendQueue chan interface{}
	team      string
	id        string
}

type paddlePositions struct {
	leftPaddle  float64
	rightPaddle float64
}

type WebSocketHandler struct {
	Upgrader        websocket.Upgrader
	LeftPaddleData  paddleData
	RightPaddleData paddleData
	InitLeftPaddle  float64
	InitRightPaddle float64
	BallVar         ball.Ball
	CanvasVar       canvas.Canvas
	PaddleVar       paddle.Paddle
	Mu              sync.Mutex
	Connections     map[string]*Client
	ConnToId        map[*websocket.Conn]string
	BallRunning     bool
	BallVisible     bool
}

// ball constants
const (
	initialBallDx = 20
	initialBallDy = 0
	ballRadius    = 8
)

// paddle constants
const (
	maxSpeed     = 10.0
	acceleration = 2.0
	friction     = 0.9
)

var globalPaddlePositions = &paddlePositions{}

func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		Upgrader:    websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		Connections: make(map[string]*Client),
		ConnToId:    make(map[*websocket.Conn]string),
	}
}

func (wsh *WebSocketHandler) startBallUpdates() {

	ticker := time.NewTicker(32 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		wsh.Mu.Lock()
		if len(wsh.Connections) == 0 {
			log.Println("No active connections, skipping update") // Debugging log
			wsh.Mu.Unlock()
			continue
		}
		wsh.Mu.Unlock()

		wsh.updateBallPosition()

		wsh.Mu.Lock()
		message := map[string]interface{}{
			"ball": map[string]float64{
				"x":      wsh.BallVar.X,
				"y":      wsh.BallVar.Y,
				"radius": wsh.BallVar.Radius,
			},
		}
		wsh.Mu.Unlock()

		wsh.broadcastToAll(message)
	}
}

func (wsh *WebSocketHandler) disconnectPlayer(conn *websocket.Conn) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	clientId, exists := wsh.ConnToId[conn]
	if !exists {
		return
	}

	client, exists := wsh.Connections[clientId]

	if client.team == "left" {
		wsh.LeftPaddleData.players--
	} else {
		wsh.RightPaddleData.players--
	}

	close(client.sendQueue)
	delete(wsh.Connections, clientId)
	delete(wsh.ConnToId, conn)

	conn.Close()
}

func (wsh *WebSocketHandler) broadcastPaddlePositions() {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	// Prepare game state with current paddle positions
	gameState := map[string]float64{
		"leftPaddleData":  wsh.LeftPaddleData.position,
		"rightPaddleData": wsh.RightPaddleData.position,
	}

	for _, client := range wsh.Connections {
		select {
		case client.sendQueue <- gameState:
		default:
			log.Println("Dropping message, send queue full for client")
		}
	}
}

func (wsh *WebSocketHandler) broadcastToAll(message interface{}) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	for _, client := range wsh.Connections {
		select {
		case client.sendQueue <- message:
			// log.Println("Message sent to client:", conn.RemoteAddr())
		default:
			log.Println("Dropping message, send queue full for client")
		}
	}
}

func (wsh *WebSocketHandler) updateBallPosition() {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	// update ball position
	wsh.BallVar.X += wsh.BallVar.Dx
	wsh.BallVar.Y += wsh.BallVar.Dy

	maxWidth := wsh.CanvasVar.Width
	maxHeight := wsh.CanvasVar.Height
	ballRadius := wsh.BallVar.Radius

	// wall collision (top & bottom)
	if wsh.BallVar.Y-ballRadius <= 0 || wsh.BallVar.Y+ballRadius >= maxHeight {
		wsh.BallVar.Dy *= -1
	}

	// wall collision (left & right)
	if wsh.BallVar.X-ballRadius <= 0 || wsh.BallVar.X+ballRadius >= maxWidth {
		wsh.BallVar.Dx *= -1
	}

	// paddle collision logic
	wsh.checkPaddleCollision()
}

func (wsh *WebSocketHandler) updatePaddlePositions(client *Client, direction string) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	var paddle *paddleData
	var globalPosition *float64

	if client.team == "left" {
		paddle = &wsh.LeftPaddleData
		globalPosition = &globalPaddlePositions.leftPaddle
	} else {
		paddle = &wsh.RightPaddleData
		globalPosition = &globalPaddlePositions.rightPaddle
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
	} else if newPosition+wsh.PaddleVar.Height > wsh.CanvasVar.Height {
		newPosition = wsh.CanvasVar.Height - wsh.PaddleVar.Height
		paddle.velocity = 0
	}

	paddle.movementSum += paddle.velocity

	if paddle.players > 0 {
		paddle.position = paddle.movementSum / float64(paddle.players)
		paddle.movementSum = 0
	} else {
		paddle.position = 0
		paddle.movementSum = 0
	}

	wsh.broadcastPaddlePositions()
}

func (wsh *WebSocketHandler) checkPaddleCollision() {
	ballRadius := wsh.BallVar.Radius

	leftPaddleRight := wsh.PaddleVar.Width
	leftPaddleTop := float64(globalPaddlePositions.leftPaddle)
	leftPaddleBottom := leftPaddleTop + float64(wsh.PaddleVar.Height)

	rightPaddleLeft := wsh.CanvasVar.Width - wsh.PaddleVar.Width
	rightPaddleTop := float64(globalPaddlePositions.rightPaddle)
	rightPaddleBottom := rightPaddleTop + float64(wsh.PaddleVar.Height)

	ballSpeed := math.Hypot(wsh.BallVar.Dx, wsh.BallVar.Dy)
	maxBounceAngle := math.Pi / 3 // 60 degrees max

	if wsh.BallVar.X-ballRadius <= leftPaddleRight &&
		wsh.BallVar.Y >= leftPaddleTop &&
		wsh.BallVar.Y <= leftPaddleBottom {
		log.Println("collision with the left paddle detected, paddle height top and bottom", leftPaddleTop, leftPaddleBottom)

		relativePosition := (wsh.BallVar.Y - (leftPaddleTop + float64(wsh.PaddleVar.Height)/2)) / (float64(wsh.PaddleVar.Height) / 2)
		bounceAngle := relativePosition * maxBounceAngle
		wsh.BallVar.Dx = math.Abs(ballSpeed * math.Cos(bounceAngle))
		wsh.BallVar.Dy = ballSpeed * math.Sin(bounceAngle)
		wsh.BallVar.Dy += randomVariation()
		wsh.BallVar.X = leftPaddleRight + ballRadius
	}

	if wsh.BallVar.X+ballRadius >= rightPaddleLeft &&
		wsh.BallVar.Y >= rightPaddleTop &&
		wsh.BallVar.Y <= rightPaddleBottom {
		log.Println("collision with the right paddle detected, paddle height top and bottom", rightPaddleTop, rightPaddleBottom)

		relativePosition := (wsh.BallVar.Y - (rightPaddleTop + float64(wsh.PaddleVar.Height)/2)) / (float64(wsh.PaddleVar.Height) / 2)
		bounceAngle := relativePosition * maxBounceAngle
		wsh.BallVar.Dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
		wsh.BallVar.Dy = ballSpeed * math.Sin(bounceAngle)
		wsh.BallVar.Dy += randomVariation()
		wsh.BallVar.X = rightPaddleLeft - ballRadius
	}
}

func randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
}

func (wsh *WebSocketHandler) resetBall() {
	wsh.BallVar.X = wsh.CanvasVar.Width / 2
	wsh.BallVar.Y = wsh.CanvasVar.Height / 2

	// reverse the direction randomly for variation
	wsh.BallVar.Dx = -wsh.BallVar.Dx
}

func (wsh *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.Upgrader.Upgrade(w, r, nil)
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

	log.Println("global paddle positions", globalPaddlePositions)

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

	wsh.Mu.Lock()

	wsh.Connections[clientId] = client
	wsh.ConnToId[conn] = clientId

	log.Println("Total number of connections: ", len(wsh.Connections))
	wsh.Mu.Unlock()

	var initialized bool

	// Handle incoming messages
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from the client: %s", err)
			wsh.disconnectPlayer(conn)
			return
		}

		//log.Printf("Message received: %s", p)

		var msg struct {
			Type         string  `json:"type"`
			Direction    string  `json:"direction"`
			Paddle       string  `json:"paddle"`
			Width        float64 `json:width`
			Height       float64 `json:height`
			PaddleHeight float64 `json:paddleHeight`
			PaddleWidth  float64 `json:paddleWidth`
		}

		err = json.Unmarshal(p, &msg)
		if err != nil {
			log.Printf("Error unmarshalling message: %s", err)
			// Send JSON error response
			client.sendQueue <- map[string]string{"error": "Invalid message format"}
			continue
		}

		log.Println("this is the width from the message", msg.Width)
		log.Println("this is the height form the message", msg.Height)

		if !initialized && msg.Width > 0 && msg.Height > 0 {
			wsh.Mu.Lock()

			if len(wsh.Connections)%2 == 0 {
				client.team = "left"
				wsh.LeftPaddleData.players++
			} else {
				client.team = "right"
				wsh.RightPaddleData.players++
			}

			wsh.BallVar = ball.Ball{
				X:       msg.Width / 2,
				Y:       msg.Height / 2,
				Dx:      -10,
				Dy:      0,
				Radius:  ballRadius,
				Visible: true,
			}

			log.Println(wsh.BallVar)

			wsh.CanvasVar.Width = msg.Width
			wsh.PaddleVar.Width = msg.PaddleWidth
			wsh.PaddleVar.Height = msg.PaddleHeight
			wsh.CanvasVar.Height = msg.Height

			globalPaddlePositions.leftPaddle = wsh.LeftPaddleData.position
			globalPaddlePositions.rightPaddle = wsh.RightPaddleData.position

			if len(wsh.Connections) == 1 {
				wsh.LeftPaddleData.position = (msg.Height / 2) - (msg.PaddleHeight / 2)
				wsh.RightPaddleData.position = (msg.Height / 2) - (msg.PaddleHeight / 2)
				wsh.BallRunning = false
				wsh.BallVisible = false
			}

			if !wsh.BallRunning && len(wsh.Connections) > 1 {
				wsh.BallRunning = true
				go wsh.startBallUpdates()
			}

			wsh.Mu.Unlock()
			initialized = true

			initialGameState := map[string]interface{}{
				"leftPaddleData":  wsh.LeftPaddleData.position,
				"rightPaddleData": wsh.RightPaddleData.position,
				"yourTeam":        client.team,
				"clients":         len(wsh.Connections),
			}

			log.Println(initialGameState)

			client.sendQueue <- initialGameState

			continue

		}

		client.sendQueue <- map[string]string{"status": "Message processed"}

		var movement float64
		if msg.Direction == "up" {
			movement = -30
		} else if msg.Direction == "down" {
			movement = 30
		}

		wsh.Mu.Lock()
		log.Println("updates of global positions, left paddle and right paddle  ", globalPaddlePositions, wsh.LeftPaddleData.position, wsh.RightPaddleData.position)
		if client.team == "left" {
			newLeftPaddlePos := globalPaddlePositions.leftPaddle + movement

			if newLeftPaddlePos >= 0 && newLeftPaddlePos+wsh.PaddleVar.Height <= wsh.CanvasVar.Height {
				globalPaddlePositions.leftPaddle = newLeftPaddlePos
			}

			wsh.LeftPaddleData.movementSum += movement
			if wsh.LeftPaddleData.players > 0 {
				wsh.LeftPaddleData.position = wsh.LeftPaddleData.movementSum / float64(wsh.LeftPaddleData.players)
				wsh.RightPaddleData.position = 0
				wsh.LeftPaddleData.movementSum = 0
			} else {
				wsh.LeftPaddleData.position = 0
				wsh.LeftPaddleData.movementSum = 0
			}
		} else {
			newRightPaddlePos := globalPaddlePositions.rightPaddle + movement

			if newRightPaddlePos >= 0 && newRightPaddlePos+wsh.PaddleVar.Height <= wsh.CanvasVar.Height {
				globalPaddlePositions.rightPaddle = newRightPaddlePos
			}

			wsh.RightPaddleData.movementSum += movement
			if wsh.RightPaddleData.players > 0 {
				wsh.RightPaddleData.position = wsh.RightPaddleData.movementSum / float64(wsh.RightPaddleData.players)
				wsh.LeftPaddleData.position = 0
				wsh.RightPaddleData.movementSum = 0
			} else {
				wsh.RightPaddleData.position = 0
				wsh.RightPaddleData.movementSum = 0
			}
		}
		wsh.Mu.Unlock()
		wsh.broadcastPaddlePositions()
	}
}
