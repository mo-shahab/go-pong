package wsserver

import (
	"sync"
	"github.com/gorilla/websocket"
	"github.com/mo-shahab/go-pong/ball"
	"github.com/mo-shahab/go-pong/canvas"
	"github.com/mo-shahab/go-pong/client"
	"github.com/mo-shahab/go-pong/paddle"
	"github.com/mo-shahab/go-pong/room"
	"github.com/mo-shahab/go-pong/scores"
	"github.com/mo-shahab/go-pong/game"
)

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
	Connections     map[string]*client.Client
	ConnToId        map[*websocket.Conn]string
	BallRunning     bool
	BallVisible     bool
	Scores          scores.Scores
	RoomManager     *room.RoomManager
	WaitingRooms map[string]*room.WaitingRoomState
	GameEngine *game.Engine
}

type paddleData struct {
	movementSum float64
	velocity    float64
	players     int
	position    float64
}

type paddlePositions struct {
	leftPaddle  float64
	rightPaddle float64
}
