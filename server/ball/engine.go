package ball

import (
    "github.com/mo-shahab/go-pong/canvas"
    "github.com/mo-shahab/go-pong/client"
	"github.com/mo-shahab/go-pong/scores"
	"github.com/mo-shahab/go-pong/paddle"
    "time"
    "sync"
    "log"
	"math/rand"
)

func resetBall(
    ball *Ball,
    canvas *canvas.Canvas,
    directionX int,
    broadcast func(interface{}),
) {
	ball.X = canvas.Width / 2
	ball.Y = canvas.Height / 2

	baseSpeed := 10
	ball.Dx = float64(directionX) * float64(baseSpeed)
	ball.Dy = (rand.Float64() - 0.5) * 5.0
}

func checkBallOutOfBounds(
    ball *Ball,
    score *scores.Scores,
    mutex *sync.Mutex,
    canvas *canvas.Canvas,
    broadcast func(interface{}),
) {
    timer := time.NewTimer(3 * time.Second)
    defer timer.Stop()

	mutex.Lock()

	ballRadius := ball.Radius
	scored := false
	scoreUpdate := map[string]interface{}{}

    whoScored := ""

	// ball colliding with the left wall
	if ball.X-ballRadius <= 0 {
		// Right players score
		score.RightScores++
		log.Println("Right Player Scored! Score:  ", score.RightScores, "-", score.LeftScores)
		resetBall(ball, canvas, 1, broadcast)
		scored = true
        whoScored = "Right"
	}

	// ball colliding with the left wall
	if ball.X+ballRadius >= canvas.Width {
		// Left players score
		score.LeftScores++
		log.Println("Left Player Scored! Score:  ", score.RightScores, "-", score.LeftScores)
		resetBall(ball, canvas, -1, broadcast)
		scored = true
        whoScored = "Left"
	}

	if scored {

		scoreUpdate = map[string]interface{}{
			"type":       "score",
			"leftScore":  score.LeftScores,
			"rightScore": score.RightScores,
			"scored": whoScored,
		}
	}

	// Release the lock before broadcasting
	mutex.Unlock()

	// Broadcast outside of the lock if we scored
	if scored {
		broadcast(scoreUpdate)
        log.Println("timer started")
        <-timer.C
        log.Println("timer stopped")
	}
}

func updateBallPosition(
    ball *Ball,
    score *scores.Scores,
    canvas *canvas.Canvas,
    mutex *sync.Mutex,
    broadcast func(interface{}),
) {
	mutex.Lock()

	// update ball position
	ball.X += ball.Dx
	ball.Y += ball.Dy

	// maxWidth := canvas.Width
	maxHeight := canvas.Height
	ballRadius := ball.Radius

	// wall collision (top & bottom)
	if ball.Y - ballRadius <= 0 || ball.Y + ballRadius >= maxHeight {
		ball.Dy *= -1
	}

	/*
	   --DEPRECATED-- (now since scoring is there, this dont make sense)

	   if ball.X-ballRadius <= 0 || ball.X+ballRadius >= maxWidth {
	   ball.Dx *= -1
	   }

	*/

	// paddle collision logic
	paddle.handlePaddleCollision()
	mutex.Unlock()

	// check if there is any scoring
    checkBallOutOfBounds(ball, score, mutex, canvas, broadcast)
}

func StartBallUpdates(
    ball *Ball,
    score *scores.Scores,
    canvas *canvas.Canvas,
    mutex *sync.Mutex,
    connections map[string]*client.Client,
    broadcast func(interface{}),
) {

	ticker := time.NewTicker(32 * time.Millisecond)
	defer ticker.Stop()

	for {
		<-ticker.C

		mutex.Lock()
		if len(connections) == 0 {
			mutex.Unlock()
			continue
		}
		mutex.Unlock()

        updateBallPosition(ball, score, canvas, mutex, broadcast)

		mutex.Lock()
		message := map[string]interface{}{
			"ball": map[string]float64{
				"x":      ball.X,
				"y":      ball.Y,
				"radius": ball.Radius,
			},
		}
		mutex.Unlock()

		broadcast(message)
	}
}

