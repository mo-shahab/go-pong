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
	"github.com/mo-shahab/go-pong/client"
	"github.com/mo-shahab/go-pong/paddle"
	pb "github.com/mo-shahab/go-pong/proto"
	"github.com/mo-shahab/go-pong/scores"
	"google.golang.org/protobuf/proto"
)

// GameState holds the current state of a game session
type GameState struct {
	Ball             ball.Ball
	Canvas           canvas.Canvas
	Paddle           paddle.Paddle
	Scores           scores.Scores
	LeftPaddlePos    float64
	RightPaddlePos   float64
	BallRunning      bool
	BallVisible      bool
	Initialized      bool
	mu               sync.RWMutex
}

// PaddleManager handles paddle movement and collision logic
type PaddleManager struct {
	leftPaddleData  paddleData
	rightPaddleData paddleData
	gameState       *GameState
	mu              sync.RWMutex
}

// BallManager handles ball physics and collision detection
type BallManager struct {
	gameState     *GameState
	paddleManager *PaddleManager
	mu            sync.RWMutex
}

// GameEngine coordinates all game subsystems
type GameEngine struct {
	gameState     *GameState
	paddleManager *PaddleManager
	ballManager   *BallManager
	broadcaster   MessageBroadcaster
	ticker        *time.Ticker
	stopChan      chan struct{}
	mu            sync.RWMutex
}

// MessageBroadcaster interface for sending messages to clients
type MessageBroadcaster interface {
	BroadcastToAll(message []byte)
	BroadcastToRoom(roomId string, message []byte)
}

// paddleData represents paddle state for a team
type paddleData struct {
	movementSum float64
	velocity    float64
	players     int
	position    float64
}

// Game constants
const (
	// Ball constants
	InitialBallDx = 20
	InitialBallDy = 0
	BallRadius    = 8
	
	// Paddle constants
	MaxSpeed     = 10.0
	Acceleration = 2.0
	Friction     = 0.9
	
	// Game loop
	TickRate = 32 * time.Millisecond
)

// NewGameEngine creates a new game engine instance
func NewGameEngine(broadcaster MessageBroadcaster) *GameEngine {
	gameState := &GameState{}
	paddleManager := &PaddleManager{
		gameState: gameState,
	}
	ballManager := &BallManager{
		gameState:     gameState,
		paddleManager: paddleManager,
	}
	
	return &GameEngine{
		gameState:     gameState,
		paddleManager: paddleManager,
		ballManager:   ballManager,
		broadcaster:   broadcaster,
		stopChan:      make(chan struct{}),
	}
}

// Initialize sets up the game with initial parameters
func (ge *GameEngine) Initialize(width, height, paddleWidth, paddleHeight float64) {
	ge.gameState.mu.Lock()
	defer ge.gameState.mu.Unlock()
	
	ge.gameState.Ball = ball.Ball{
		X:       width / 2,
		Y:       height / 2,
		Dx:      -10,
		Dy:      0,
		Radius:  BallRadius,
		Visible: true,
	}
	
	ge.gameState.Canvas = canvas.Canvas{
		Width:  width,
		Height: height,
	}
	
	ge.gameState.Paddle = paddle.Paddle{
		Width:  paddleWidth,
		Height: paddleHeight,
	}
	
	// Initialize paddle positions to center
	centerY := (height / 2) - (paddleHeight / 2)
	ge.gameState.LeftPaddlePos = centerY
	ge.gameState.RightPaddlePos = centerY
	
	ge.paddleManager.leftPaddleData.position = centerY
	ge.paddleManager.rightPaddleData.position = centerY
	
	ge.gameState.Initialized = true
	
	log.Println("Game engine initialized")
}

// Start begins the game loop
func (ge *GameEngine) Start() {
	ge.mu.Lock()
	if ge.ticker != nil {
		ge.mu.Unlock()
		return // Already running
	}
	
	ge.ticker = time.NewTicker(TickRate)
	ge.gameState.BallRunning = true
	ge.mu.Unlock()
	
	log.Println("Starting game engine")
	go ge.gameLoop()
}

// Stop halts the game loop
func (ge *GameEngine) Stop() {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	
	if ge.ticker != nil {
		ge.ticker.Stop()
		ge.ticker = nil
		close(ge.stopChan)
		ge.stopChan = make(chan struct{})
		ge.gameState.BallRunning = false
		log.Println("Game engine stopped")
	}
}

// gameLoop runs the main game update cycle
func (ge *GameEngine) gameLoop() {
	for {
		select {
		case <-ge.stopChan:
			return
		case <-ge.ticker.C:
			ge.update()
		}
	}
}

// update handles one game loop iteration
func (ge *GameEngine) update() {
	ge.ballManager.updateBall()
	ge.broadcastGameState()
}

// MovePaddle handles paddle movement for a specific team
func (ge *GameEngine) MovePaddle(team, direction string) {
	ge.paddleManager.updatePaddlePosition(team, direction)
}

// GetGameState returns the current game state safely
func (ge *GameEngine) GetGameState() GameStateSnapshot {
	ge.gameState.mu.RLock()
	defer ge.gameState.mu.RUnlock()
	
	return GameStateSnapshot{
		Ball:           ge.gameState.Ball,
		LeftPaddlePos:  ge.gameState.LeftPaddlePos,
		RightPaddlePos: ge.gameState.RightPaddlePos,
		Scores:         ge.gameState.Scores,
		BallRunning:    ge.gameState.BallRunning,
		BallVisible:    ge.gameState.BallVisible,
		Initialized:    ge.gameState.Initialized,
	}
}

// GameStateSnapshot represents a point-in-time snapshot of game state
type GameStateSnapshot struct {
	Ball           ball.Ball
	LeftPaddlePos  float64
	RightPaddlePos float64
	Scores         scores.Scores
	BallRunning    bool
	BallVisible    bool
	Initialized    bool
}

// broadcastGameState sends current game state to all clients
func (ge *GameEngine) broadcastGameState() {
	snapshot := ge.GetGameState()
	
	ballObject := &pb.Ball{
		X:      snapshot.Ball.X,
		Y:      snapshot.Ball.Y,
		Radius: snapshot.Ball.Radius,
	}
	
	ballPositionMessage := &pb.BallPositionMessage{
		Ball: ballObject,
	}
	
	wrappedMessage := &pb.Message{
		Type: pb.MsgType_ball_position,
		MessageType: &pb.Message_BallPosition{
			BallPosition: ballPositionMessage,
		},
	}
	
	message, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to encode ball message: %v", err)
		return
	}
	
	ge.broadcaster.BroadcastToAll(message)
}

// UpdatePaddlePosition updates paddle position based on player input
func (pm *PaddleManager) updatePaddlePosition(team, direction string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var paddle *paddleData
	var globalPosition *float64
	
	pm.gameState.mu.Lock()
	defer pm.gameState.mu.Unlock()
	
	if team == "left" {
		paddle = &pm.leftPaddleData
		globalPosition = &pm.gameState.LeftPaddlePos
	} else {
		paddle = &pm.rightPaddleData
		globalPosition = &pm.gameState.RightPaddlePos
	}
	
	// Update velocity based on direction
	switch direction {
	case "up":
		paddle.velocity -= Acceleration
	case "down":
		paddle.velocity += Acceleration
	default:
		paddle.velocity *= Friction
	}
	
	// Clamp velocity
	if paddle.velocity > MaxSpeed {
		paddle.velocity = MaxSpeed
	} else if paddle.velocity < -MaxSpeed {
		paddle.velocity = -MaxSpeed
	}
	
	// Calculate new position
	newPosition := *globalPosition + paddle.velocity
	
	// Boundary checking
	if newPosition < 0 {
		newPosition = 0
		paddle.velocity = 0
	} else if newPosition+pm.gameState.Paddle.Height > pm.gameState.Canvas.Height {
		newPosition = pm.gameState.Canvas.Height - pm.gameState.Paddle.Height
		paddle.velocity = 0
	}
	
	// Update global position (source of truth)
	*globalPosition = newPosition
	
	// Update paddle data for averaging if multiple players
	paddle.movementSum += paddle.velocity
	if paddle.players > 0 {
		paddle.position = paddle.movementSum / float64(paddle.players)
		paddle.movementSum = 0
	} else {
		paddle.position = 0
		paddle.movementSum = 0
	}
}

// AddPlayerToPaddle adds a player to a paddle team
func (pm *PaddleManager) AddPlayerToPaddle(team string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if team == "left" {
		pm.leftPaddleData.players++
	} else {
		pm.rightPaddleData.players++
	}
}

// RemovePlayerFromPaddle removes a player from a paddle team
func (pm *PaddleManager) RemovePlayerFromPaddle(team string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if team == "left" && pm.leftPaddleData.players > 0 {
		pm.leftPaddleData.players--
	} else if team == "right" && pm.rightPaddleData.players > 0 {
		pm.rightPaddleData.players--
	}
}

// updateBall handles ball physics and collision detection
func (bm *BallManager) updateBall() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	
	bm.gameState.mu.Lock()
	defer bm.gameState.mu.Unlock()
	
	// Update ball position
	bm.gameState.Ball.X += bm.gameState.Ball.Dx
	bm.gameState.Ball.Y += bm.gameState.Ball.Dy
	
	// Wall collision (top & bottom)
	ballRadius := bm.gameState.Ball.Radius
	maxHeight := bm.gameState.Canvas.Height
	
	if bm.gameState.Ball.Y-ballRadius <= 0 || bm.gameState.Ball.Y+ballRadius >= maxHeight {
		bm.gameState.Ball.Dy *= -1
	}
	
	// Paddle collision
	bm.handlePaddleCollision()
	
	// Check for scoring
	bm.checkBallOutOfBounds()
}

// handlePaddleCollision detects and handles ball-paddle collisions
func (bm *BallManager) handlePaddleCollision() {
	ballRadius := bm.gameState.Ball.Radius
	
	// Left paddle collision
	leftPaddleRight := bm.gameState.Paddle.Width
	leftPaddleTop := bm.gameState.LeftPaddlePos
	leftPaddleBottom := leftPaddleTop + bm.gameState.Paddle.Height
	
	// Right paddle collision
	rightPaddleLeft := bm.gameState.Canvas.Width - bm.gameState.Paddle.Width
	rightPaddleTop := bm.gameState.RightPaddlePos
	rightPaddleBottom := rightPaddleTop + bm.gameState.Paddle.Height
	
	ballSpeed := math.Hypot(bm.gameState.Ball.Dx, bm.gameState.Ball.Dy)
	maxBounceAngle := math.Pi / 3 // 60 degrees max
	
	// Left paddle collision
	if bm.gameState.Ball.X-ballRadius <= leftPaddleRight &&
		bm.gameState.Ball.Y >= leftPaddleTop &&
		bm.gameState.Ball.Y <= leftPaddleBottom &&
		bm.gameState.Ball.Dx < 0 {
		
		relativePosition := (bm.gameState.Ball.Y - (leftPaddleTop + bm.gameState.Paddle.Height/2)) / (bm.gameState.Paddle.Height / 2)
		bounceAngle := relativePosition * maxBounceAngle
		
		bm.gameState.Ball.Dx = math.Abs(ballSpeed * math.Cos(bounceAngle))
		bm.gameState.Ball.Dy = ballSpeed * math.Sin(bounceAngle)
		bm.gameState.Ball.Dy += bm.randomVariation()
		bm.gameState.Ball.X = leftPaddleRight + ballRadius
	}
	
	// Right paddle collision
	if bm.gameState.Ball.X+ballRadius >= rightPaddleLeft &&
		bm.gameState.Ball.Y >= rightPaddleTop &&
		bm.gameState.Ball.Y <= rightPaddleBottom &&
		bm.gameState.Ball.Dx > 0 {
		
		relativePosition := (bm.gameState.Ball.Y - (rightPaddleTop + bm.gameState.Paddle.Height/2)) / (bm.gameState.Paddle.Height / 2)
		bounceAngle := relativePosition * maxBounceAngle
		
		bm.gameState.Ball.Dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
		bm.gameState.Ball.Dy = ballSpeed * math.Sin(bounceAngle)
		bm.gameState.Ball.Dy += bm.randomVariation()
		bm.gameState.Ball.X = rightPaddleLeft - ballRadius
	}
}

// checkBallOutOfBounds handles scoring when ball goes out of bounds
func (bm *BallManager) checkBallOutOfBounds() {
	ballRadius := bm.gameState.Ball.Radius
	scored := false
	whoScored := ""
	
	// Ball hit left wall - right player scores
	if bm.gameState.Ball.X-ballRadius <= 0 {
		bm.gameState.Scores.RightScores++
		bm.resetBall(1)
		scored = true
		whoScored = "Right"
		log.Printf("Right Player Scored! Score: %d-%d", 
			bm.gameState.Scores.RightScores, bm.gameState.Scores.LeftScores)
	}
	
	// Ball hit right wall - left player scores
	if bm.gameState.Ball.X+ballRadius >= bm.gameState.Canvas.Width {
		bm.gameState.Scores.LeftScores++
		bm.resetBall(-1)
		scored = true
		whoScored = "Left"
		log.Printf("Left Player Scored! Score: %d-%d", 
			bm.gameState.Scores.RightScores, bm.gameState.Scores.LeftScores)
	}
	
	if scored {
		bm.broadcastScore(whoScored)
		// Add small delay after scoring
		time.Sleep(100 * time.Millisecond)
	}
}

// resetBall resets ball to center with specified direction
func (bm *BallManager) resetBall(directionX int) {
	bm.gameState.Ball.X = bm.gameState.Canvas.Width / 2
	bm.gameState.Ball.Y = bm.gameState.Canvas.Height / 2
	
	baseSpeed := 10.0
	bm.gameState.Ball.Dx = float64(directionX) * baseSpeed
	bm.gameState.Ball.Dy = (rand.Float64() - 0.5) * 5.0
}

// broadcastScore sends score update to all clients
func (bm *BallManager) broadcastScore(whoScored string) {
	// This would need to be implemented with access to the broadcaster
	// For now, just log - you'll need to pass broadcaster to BallManager
	log.Printf("Score update: Left %d - Right %d", 
		bm.gameState.Scores.LeftScores, bm.gameState.Scores.RightScores)
}

// randomVariation adds some randomness to ball movement
func (bm *BallManager) randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
}
