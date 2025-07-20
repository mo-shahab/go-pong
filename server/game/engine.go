// game/engine.go

package game

import (
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/mo-shahab/go-pong/ball"
	"github.com/mo-shahab/go-pong/canvas"
	"github.com/mo-shahab/go-pong/paddle"
	"github.com/mo-shahab/go-pong/scores"
	pb "github.com/mo-shahab/go-pong/proto"
)

// Constants
const (
	InitialBallDx = 20
	InitialBallDy = 0
	BallRadius    = 8
	MaxSpeed      = 10.0
	Acceleration  = 2.0
	Friction      = 0.9
)

// GameState represents the current state of a game
type GameState struct {
	Ball             ball.Ball
	Canvas           canvas.Canvas
	Paddle           paddle.Paddle
	LeftPaddleData   PaddleData
	RightPaddleData  PaddleData
	Scores           scores.Scores
	BallRunning      bool
	BallVisible      bool
	Initialized      bool
	Running          bool
	Mu               sync.RWMutex
	paddlePositions  PaddlePositions
	ballUpdateTicker *time.Ticker
	stopBallUpdates  chan bool
}

// GameEventHandler defines the interface for handling game events
type GameEventHandler interface {
	OnBallPositionUpdate(ball *pb.Ball)
	OnScoreUpdate(leftScore, rightScore int32, whoScored string)
	OnGameStateUpdate(leftPaddle, rightPaddle float64)
}

// Engine represents the game engine
type Engine struct {
	games   map[string]*GameState
	mu      sync.RWMutex
	handler GameEventHandler
}

// NewEngine creates a new game engine
func NewEngine(handler GameEventHandler) *Engine {
	return &Engine{
		games:   make(map[string]*GameState),
		handler: handler,
	}
}

// CreateGame creates a new game instance
func (e *Engine) CreateGame(gameID string) *GameState {
	e.mu.Lock()
	defer e.mu.Unlock()

	game := &GameState{
		paddlePositions: PaddlePositions{},
		stopBallUpdates: make(chan bool, 1),
	}

	e.games[gameID] = game
	return game
}

// GetGame retrieves a game by ID
func (e *Engine) GetGame(gameID string) (*GameState, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	game, exists := e.games[gameID]
	return game, exists
}

// RemoveGame removes a game from the engine
func (e *Engine) RemoveGame(gameID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if game, exists := e.games[gameID]; exists {
		game.StopBallUpdates()
		delete(e.games, gameID)
	}
}

// InitializeGame initializes the game with canvas and paddle dimensions
func (gs *GameState) Initialize(width, height, paddleWidth, paddleHeight float64) {
	gs.Mu.Lock()
	defer gs.Mu.Unlock()

	gs.Ball = ball.Ball{
		X:       width / 2,
		Y:       height / 2,
		Dx:      -10,
		Dy:      0,
		Radius:  BallRadius,
		Visible: true,
	}

	gs.Canvas = canvas.Canvas{
		Width:  width,
		Height: height,
	}

	gs.Paddle = paddle.Paddle{
		Width:  paddleWidth,
		Height: paddleHeight,
	}

	// Initialize paddle positions to center
	centerY := (height / 2) - (paddleHeight / 2)
	gs.LeftPaddleData.Position = centerY
	gs.RightPaddleData.Position = centerY
	gs.paddlePositions.LeftPaddle = centerY
	gs.paddlePositions.RightPaddle = centerY

	gs.Initialized = true
}

// StartBallUpdates begins the ball update loop
func (gs *GameState) StartBallUpdates(engine *Engine, gameID string) {
	gs.Mu.Lock()
	if gs.BallRunning {
		gs.Mu.Unlock()
		return
	}
	gs.BallRunning = true
	gs.ballUpdateTicker = time.NewTicker(32 * time.Millisecond)
	gs.Mu.Unlock()

	go func() {
		defer gs.ballUpdateTicker.Stop()

		for {
			select {
			case <-gs.ballUpdateTicker.C:
				gs.updateBallPosition(engine, gameID)
			case <-gs.stopBallUpdates:
				return
			}
		}
	}()
}

// StopBallUpdates stops the ball update loop
func (gs *GameState) StopBallUpdates() {
	gs.Mu.Lock()
	defer gs.Mu.Unlock()

	if gs.BallRunning {
		gs.BallRunning = false
		if gs.ballUpdateTicker != nil {
			gs.ballUpdateTicker.Stop()
		}
		select {
		case gs.stopBallUpdates <- true:
		default:
		}
	}
}

// MovePaddle handles paddle movement for a team
func (gs *GameState) MovePaddle(team string, direction string) (float64, float64) {
	gs.Mu.Lock()
	defer gs.Mu.Unlock()

	var movement float64
	switch direction {
	case "up":
		movement = -30
	case "down":
		movement = 30
	default:
		return gs.paddlePositions.LeftPaddle, gs.paddlePositions.RightPaddle
	}

	if team == "left" {
		newPos := gs.paddlePositions.LeftPaddle + movement
		if newPos >= 0 && newPos+gs.Paddle.Height <= gs.Canvas.Height {
			gs.paddlePositions.LeftPaddle = newPos
		}

		gs.LeftPaddleData.MovementSum += movement
		if gs.LeftPaddleData.Players > 0 {
			gs.LeftPaddleData.Position = gs.LeftPaddleData.MovementSum / float64(gs.LeftPaddleData.Players)
			gs.LeftPaddleData.MovementSum = 0
		}
	} else {
		newPos := gs.paddlePositions.RightPaddle + movement
		if newPos >= 0 && newPos+gs.Paddle.Height <= gs.Canvas.Height {
			gs.paddlePositions.RightPaddle = newPos
		}

		gs.RightPaddleData.MovementSum += movement
		if gs.RightPaddleData.Players > 0 {
			gs.RightPaddleData.Position = gs.RightPaddleData.MovementSum / float64(gs.RightPaddleData.Players)
			gs.RightPaddleData.MovementSum = 0
		}
	}

	return gs.paddlePositions.LeftPaddle, gs.paddlePositions.RightPaddle
}

// AddPlayer adds a player to a team
func (gs *GameState) AddPlayer(team string) {
	gs.Mu.Lock()
	defer gs.Mu.Unlock()

	if team == "left" {
		gs.LeftPaddleData.Players++
	} else {
		gs.RightPaddleData.Players++
	}
}

// RemovePlayer removes a player from a team
func (gs *GameState) RemovePlayer(team string) {
	gs.Mu.Lock()
	defer gs.Mu.Unlock()

	if team == "left" && gs.LeftPaddleData.Players > 0 {
		gs.LeftPaddleData.Players--
	} else if team == "right" && gs.RightPaddleData.Players > 0 {
		gs.RightPaddleData.Players--
	}
}

// GetGameState returns the current game state for client updates
func (gs *GameState) GetGameState() (leftPaddle, rightPaddle float64, leftScore, rightScore int32) {
	gs.Mu.RLock()
	defer gs.Mu.RUnlock()

	return gs.paddlePositions.LeftPaddle, gs.paddlePositions.RightPaddle, gs.Scores.LeftScores, gs.Scores.RightScores
}

// GetBallState returns the current ball state
func (gs *GameState) GetBallState() ball.Ball {
	gs.Mu.RLock()
	defer gs.Mu.RUnlock()

	return gs.Ball
}

// updateBallPosition updates the ball position and handles collisions
func (gs *GameState) updateBallPosition(engine *Engine, gameID string) {
	gs.Mu.Lock()

	// Update ball position
	gs.Ball.X += gs.Ball.Dx
	gs.Ball.Y += gs.Ball.Dy

	ballRadius := gs.Ball.Radius
	maxHeight := gs.Canvas.Height

	// Wall collision (top & bottom)
	if gs.Ball.Y-ballRadius <= 0 || gs.Ball.Y+ballRadius >= maxHeight {
		gs.Ball.Dy *= -1
	}

	// Paddle collision logic
	gs.handlePaddleCollision()

	// Create ball update for broadcasting
	ballUpdate := &pb.Ball{
		X:      gs.Ball.X,
		Y:      gs.Ball.Y,
		Radius: gs.Ball.Radius,
	}

	gs.Mu.Unlock()

	// Notify handler about ball position update
	if engine.handler != nil {
		engine.handler.OnBallPositionUpdate(ballUpdate)
	}

	// Check for scoring
	gs.checkBallOutOfBounds(engine, gameID)
}

// resetBall resets the ball to center position
func (gs *GameState) resetBall(directionX int) {
	gs.Ball.X = gs.Canvas.Width / 2
	gs.Ball.Y = gs.Canvas.Height / 2

	baseSpeed := 10
	gs.Ball.Dx = float64(directionX) * float64(baseSpeed)
	gs.Ball.Dy = (rand.Float64() - 0.5) * 5.0
}

// checkBallOutOfBounds checks if ball is out of bounds and handles scoring
func (gs *GameState) checkBallOutOfBounds(engine *Engine, gameID string) {
	gs.Mu.Lock()

	ballRadius := gs.Ball.Radius
	scored := false
	whoScored := ""

	// Ball colliding with left wall (right player scores)
	if gs.Ball.X-ballRadius <= 0 {
		gs.Scores.RightScores++
		log.Printf("Right Player Scored! Score: %d - %d", gs.Scores.RightScores, gs.Scores.LeftScores)
		gs.resetBall(1)
		scored = true
		whoScored = "Right"
	}

	// Ball colliding with right wall (left player scores)
	if gs.Ball.X+ballRadius >= gs.Canvas.Width {
		gs.Scores.LeftScores++
		log.Printf("Left Player Scored! Score: %d - %d", gs.Scores.LeftScores, gs.Scores.RightScores)
		gs.resetBall(-1)
		scored = true
		whoScored = "Left"
	}

	leftScore := gs.Scores.LeftScores
	rightScore := gs.Scores.RightScores

	gs.Mu.Unlock()

	// Notify handler about score update
	if scored && engine.handler != nil {
		// Add a small delay for score display
		go func() {
			time.Sleep(3 * time.Second)
			engine.handler.OnScoreUpdate(leftScore, rightScore, whoScored)
		}()
	}
}

// handlePaddleCollision handles ball collision with paddles
func (gs *GameState) handlePaddleCollision() {
	ballRadius := gs.Ball.Radius

	leftPaddleRight := gs.Paddle.Width
	leftPaddleTop := gs.paddlePositions.LeftPaddle
	leftPaddleBottom := leftPaddleTop + gs.Paddle.Height

	rightPaddleLeft := gs.Canvas.Width - gs.Paddle.Width
	rightPaddleTop := gs.paddlePositions.RightPaddle
	rightPaddleBottom := rightPaddleTop + gs.Paddle.Height

	ballSpeed := math.Hypot(gs.Ball.Dx, gs.Ball.Dy)
	maxBounceAngle := math.Pi / 3 // 60 degrees max

	// Left paddle collision
	if gs.Ball.X-ballRadius <= leftPaddleRight &&
		gs.Ball.Y >= leftPaddleTop &&
		gs.Ball.Y <= leftPaddleBottom {

		relativePosition := (gs.Ball.Y - (leftPaddleTop + gs.Paddle.Height/2)) / (gs.Paddle.Height / 2)
		bounceAngle := relativePosition * maxBounceAngle
		gs.Ball.Dx = math.Abs(ballSpeed * math.Cos(bounceAngle))
		gs.Ball.Dy = ballSpeed * math.Sin(bounceAngle)
		gs.Ball.Dy += randomVariation()
		gs.Ball.X = leftPaddleRight + ballRadius
	}

	// Right paddle collision
	if gs.Ball.X+ballRadius >= rightPaddleLeft &&
		gs.Ball.Y >= rightPaddleTop &&
		gs.Ball.Y <= rightPaddleBottom {

		relativePosition := (gs.Ball.Y - (rightPaddleTop + gs.Paddle.Height/2)) / (gs.Paddle.Height / 2)
		bounceAngle := relativePosition * maxBounceAngle
		gs.Ball.Dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
		gs.Ball.Dy = ballSpeed * math.Sin(bounceAngle)
		gs.Ball.Dy += randomVariation()
		gs.Ball.X = rightPaddleLeft - ballRadius
	}
}

// randomVariation adds random variation to ball movement
func randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
}
