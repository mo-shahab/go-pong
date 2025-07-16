package wsserver

import (
	"github.com/gorilla/websocket"
	"github.com/mo-shahab/go-pong/ball"
	"github.com/mo-shahab/go-pong/canvas"
	"github.com/mo-shahab/go-pong/paddle"
	"github.com/mo-shahab/go-pong/scores"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
	pb "github.com/mo-shahab/go-pong/proto"
	"google.golang.org/protobuf/proto"
)

type paddleData struct {
	movementSum float64
	velocity    float64
	players     int
	position    float64
}

type Client struct {
	conn      *websocket.Conn
	sendQueue chan []byte
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
	Scores          scores.Scores
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

// ---------------------------------------------------
// Broadcast functions
func (wsh *WebSocketHandler) broadcastToAll(message []byte) {
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

func (wsh *WebSocketHandler) broadcastBallPosition(){

}
// ---------------------------------------------------


// ---------------------------------------------------
// Ball Logic functions
func (wsh *WebSocketHandler) startBallUpdates() {

	ticker := time.NewTicker(32 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		wsh.Mu.Lock()
		if len(wsh.Connections) == 0 {
			wsh.Mu.Unlock()
			continue
		}
		wsh.Mu.Unlock()

		wsh.updateBallPosition()

		wsh.Mu.Lock()

		ballObject := &pb.Ball{
			X: wsh.BallVar.X,
			Y: wsh.BallVar.Y,
			Radius: wsh.BallVar.Radius,
		}

		ballPositionMessage := &pb.BallPositionMessage {
			Ball: ballObject,
		}

		wrappedMessage := &pb.Message {
			Type: pb.MsgType_ball_position,
			MessageType: &pb.Message_BallPosition {
				BallPosition: ballPositionMessage,
			},
		}

		message, err := proto.Marshal(wrappedMessage);

		if err != nil {
			log.Fatalln("Failed to encode the ball message: ", err)
			wsh.Mu.Unlock()
			continue
		}

		wsh.Mu.Unlock()

		wsh.broadcastToAll(message)
	}
}

func (wsh *WebSocketHandler) resetBall(directionX int) {
	wsh.BallVar.X = wsh.CanvasVar.Width / 2
	wsh.BallVar.Y = wsh.CanvasVar.Height / 2

	baseSpeed := 10
	wsh.BallVar.Dx = float64(directionX) * float64(baseSpeed)
	wsh.BallVar.Dy = (rand.Float64() - 0.5) * 5.0
}

// checks if the ball is out bounds, which would mean if the player has scored
// or not

func (wsh *WebSocketHandler) checkBallOutOfBounds() {
    timer := time.NewTimer(3 * time.Second)
    defer timer.Stop()

	wsh.Mu.Lock()

	ballRadius := wsh.BallVar.Radius
	scored := false
	scoreMessage := &pb.Message{}

    whoScored := ""

	// ball colliding with the left wall
	if wsh.BallVar.X-ballRadius <= 0 {
		// Right players score
		wsh.Scores.RightScores++
		log.Println("Right Player Scored! Score:  ", wsh.Scores.RightScores, "-", wsh.Scores.LeftScores)
		wsh.resetBall(1)
		scored = true
        whoScored = "Right"
	}

	// ball colliding with the left wall
	if wsh.BallVar.X+ballRadius >= wsh.CanvasVar.Width {
		// Left players score
		wsh.Scores.LeftScores++
		log.Println("Left Player Scored! Score:  ", wsh.Scores.RightScores, "-", wsh.Scores.LeftScores)
		wsh.resetBall(-1)
		scored = true
        whoScored = "Left"
	}

	if scored {
		scoreUpdate := &pb.ScoreMessage {
			LeftScore:  wsh.Scores.LeftScores,
			RightScore: wsh.Scores.RightScores,
			Scored: whoScored,
		}

		scoreMessage = &pb.Message{
			Type: pb.MsgType_score,
			MessageType: &pb.Message_Score{
				Score: scoreUpdate,
			},
		}
	}

	encoded, marshalErr := proto.Marshal(scoreMessage)
	if marshalErr != nil {
		log.Println("Failed to marshal ScoreMessage:", marshalErr)
	}

	// Release the lock before broadcasting
	wsh.Mu.Unlock()

	// Broadcast outside of the lock if we scored
	if scored {
		wsh.broadcastToAll(encoded)
        log.Println("timer started")
        <-timer.C
        log.Println("timer stopped")
	}
}

func (wsh *WebSocketHandler) updateBallPosition() {
	wsh.Mu.Lock()

	// update ball position
	wsh.BallVar.X += wsh.BallVar.Dx
	wsh.BallVar.Y += wsh.BallVar.Dy

	// maxWidth := wsh.CanvasVar.Width
	maxHeight := wsh.CanvasVar.Height
	ballRadius := wsh.BallVar.Radius

	// wall collision (top & bottom)
	if wsh.BallVar.Y-ballRadius <= 0 || wsh.BallVar.Y+ballRadius >= maxHeight {
		wsh.BallVar.Dy *= -1
	}

	/*
	   --DEPRECATED-- (now since scoring is there, this dont make sense)

	   if wsh.BallVar.X-ballRadius <= 0 || wsh.BallVar.X+ballRadius >= maxWidth {
	   wsh.BallVar.Dx *= -1
	   }

	*/

	// paddle collision logic
	wsh.handlePaddleCollision()
	wsh.Mu.Unlock()

	// check if there is any scoring
	wsh.checkBallOutOfBounds()
}
// ---------------------------------------------------

// ---------------------------------------------------
// Paddle Logic functions

func randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
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

	// Prepare game state with current paddle positions
	// gameState := map[string]float64{
	// 	"leftPaddleData":  globalPaddlePositions.leftPaddle,
	// 	"rightPaddleData": globalPaddlePositions.rightPaddle,
	// }

    // wsh.broadcastToAll(gameState)
	// wsh.broadcastPaddlePositions()
}

func (wsh *WebSocketHandler) handlePaddleCollision() {
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
		//log.Println("collision with the left paddle detected, paddle height top and bottom", leftPaddleTop, leftPaddleBottom)

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
		//log.Println("collision with the right paddle detected, paddle height top and bottom", rightPaddleTop, rightPaddleBottom)

		relativePosition := (wsh.BallVar.Y - (rightPaddleTop + float64(wsh.PaddleVar.Height)/2)) / (float64(wsh.PaddleVar.Height) / 2)
		bounceAngle := relativePosition * maxBounceAngle
		wsh.BallVar.Dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
		wsh.BallVar.Dy = ballSpeed * math.Sin(bounceAngle)
		wsh.BallVar.Dy += randomVariation()
		wsh.BallVar.X = rightPaddleLeft - ballRadius
	}
}
// ---------------------------------------------------

// ---------------------------------------------------
// Game Logic Functions

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
// ---------------------------------------------------

var initialized bool
var gameRunning bool

// ---------------------------------------------------
// Main Game Loop
func (wsh *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error %s when connecting to the socket", err)
		return
	}

	clientId := conn.RemoteAddr().String() + "_" + time.Now().String()

	client := &Client{
		conn:      conn,
		sendQueue: make(chan []byte, 100), // Increased buffer size
		id:        clientId,
	}

	if len(wsh.Connections) < 2 {
		log.Println("Total number of connections: ", len(wsh.Connections))
		log.Println("team assigned")
		if len(wsh.Connections)%2 == 0 {
			client.team = "left"
			wsh.LeftPaddleData.players++
		} else {
			client.team = "right"
			wsh.RightPaddleData.players++
		}
	} else {
		log.Println("Total number of connections: ", len(wsh.Connections))
		log.Println("more than 2 players")
		randomNumber := rand.Intn(100)
		log.Println("this is the randomNumber", randomNumber)
		if randomNumber%2 == 0 {
			client.team = "left"
		} else {
			client.team = "right"
		}
	}

	log.Println("this is the client", client.id, client.team)

	// a message queue, that sends the data to the client
	go func() {
		for msg := range client.sendQueue {
			err := client.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Println("Binary Message Write error (go routine sendqueue):", err)
				client.conn.Close()
				wsh.disconnectPlayer(conn)
				return
			}
		}
	}()

	wsh.Mu.Lock()

	wsh.Connections[clientId] = client
	wsh.ConnToId[conn] = clientId

	wsh.Mu.Unlock()

	// Handle incoming messages
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message from the client: %s", err)
			wsh.disconnectPlayer(conn)
			return
		}

		log.Printf("Message received: %s", p)
		
		message := &pb.Message{}
		err = proto.Unmarshal(p, message)
		log.Println("Parsed Message: ", message)
		if err != nil {
			log.Println("Error when unmarshalling protobuf binary:", err)

			errorMessage := &pb.ErrorMessage{
				Error: "Invalid protobuf format",
			}

			wrappedError := &pb.Message{
				Type: pb.MsgType_error,
				MessageType: &pb.Message_Error{
					Error: errorMessage,
				},
			}

			encoded, marshalErr := proto.Marshal(wrappedError)
			if marshalErr != nil {
				log.Println("Failed to marshal ErrorMessage:", marshalErr)
				continue
			}

			client.sendQueue <- encoded
		}


		switch message.Type {			

		case pb.MsgType_init:
			init := message.GetInit()
			log.Println("Init Message: %+v", init)

			if !initialized && init.Width > 0 && init.Height > 0 {
				wsh.Mu.Lock()

				wsh.BallVar = ball.Ball{
					X:       init.Width / 2,
					Y:       init.Height / 2,
					Dx:      -10,
					Dy:      0,
					Radius:  ballRadius,
					Visible: true,
				}

				//log.Println("global paddle positions", globalPaddlePositions)

				wsh.CanvasVar.Width = init.Width
				wsh.PaddleVar.Width = init.PaddleWidth
				wsh.PaddleVar.Height = init.PaddleHeight
				wsh.CanvasVar.Height = init.Height

				globalPaddlePositions.leftPaddle = wsh.LeftPaddleData.position
				globalPaddlePositions.rightPaddle = wsh.RightPaddleData.position

				if len(wsh.Connections) == 1 {
					wsh.LeftPaddleData.position = (init.Height / 2) - (init.PaddleHeight / 2)
					wsh.RightPaddleData.position = (init.Height / 2) - (init.PaddleHeight / 2)
					wsh.BallRunning = false
					wsh.BallVisible = false
				}

				if !wsh.BallRunning && len(wsh.Connections) > 1 {
					wsh.BallRunning = true
					go wsh.startBallUpdates()
				}

				wsh.Mu.Unlock()

				if len(wsh.Connections) == 2 {
					initialized = true
					gameRunning = true
				}

				initialGameState := &pb.InitialGameStateMessage{
					LeftPaddleData: wsh.LeftPaddleData.position,
					RightPaddleData : wsh.RightPaddleData.position,
					YourTeam : client.team,
					Clients : int32(len(wsh.Connections)),
				}

				wrappedInitialGameState := &pb.Message{
					Type: pb.MsgType_initial_game_state,
					MessageType: &pb.Message_InitialGameState{
						InitialGameState: initialGameState,
					},
				}

				encoded, marshalInitErr := proto.Marshal(wrappedInitialGameState)
				if marshalInitErr != nil {
					log.Println("Failed to marshal Initial Game State Message:", marshalInitErr)
					continue
				}

				log.Println("even after the game is initialized it still enters this block", initialized)
				client.sendQueue <- encoded

				continue

			}

		case pb.MsgType_movement:
			move := message.GetMovement()
			log.Println("Movement Message: %+v", move)

			var movement float64

			if move.Direction == "up" {
				movement = -30
			} else if move.Direction == "down" {
				movement = 30
			}

			wsh.Mu.Lock()
			//log.Println("updates of global positions, left paddle and right paddle  ",
			//globalPaddlePositions, wsh.LeftPaddleData.position, wsh.RightPaddleData.position)

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

			clients := int32(len(wsh.Connections))

			// optional fields should be sent as the address, because proto makes them pointers
			gameState := &pb.GameStateMessage{
				LeftPaddleData: &globalPaddlePositions.leftPaddle,
				RightPaddleData : &globalPaddlePositions.rightPaddle,
				YourTeam : &client.team,
				Clients : &clients,
			}

			wrappedGameState := &pb.Message{
				Type: pb.MsgType_game_state,
				MessageType: &pb.Message_GameState{
					GameState: gameState,
				},
			}

			encoded, marshalErr := proto.Marshal(wrappedGameState)
			if marshalErr != nil {
				log.Println("Failed to marshal Game State Message:", marshalErr)
				continue
			}

			client.sendQueue <- encoded
			continue
		}


		// client.sendQueue <- map[string]string{"status": "Message processed"}

		if gameRunning && initialized {
			log.Println("game is running")
			/*
				gameState := map[string]interface{}{
					"leftPaddleData":  globalPaddlePositions,
					"rightPaddleData": wsh.RightPaddleData.position,
					"yourTeam":        client.team,
					"clients":         len(wsh.Connections),
				}
			*/
		}

		// var movement float64
		// if msg.Direction == "up" {
		// 	movement = -30
		// } else if msg.Direction == "down" {
		// 	movement = 30
		// }

	}
}
