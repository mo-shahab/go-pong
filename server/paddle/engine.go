package paddle

import (
    "github.com/mo-shahab/go-pong/client"
    "github.com/mo-shahab/go-pong/canvas"
    "sync"
    "math"
    "math/rand"
)

func randomVariation() float64 {
	return (rand.Float64() - 0.5) * 2
}

func UpdatePaddlePositions(
    client *client.Client,
    leftPaddle *PaddleData,
    rightPaddle *PaddleData,
    globalPositions *PaddlePositions,
    paddle *Paddle,
    canvas *canvas.Canvas,
    mutex *sync.Mutex,
    direction string,
    broadcast func(interface{}),
) {
	mutex.Lock()
	defer mutex.Unlock()

	var paddle *PaddleData
	var globalPosition *float64

	if client.team == "left" {
		paddle = &LeftPaddleData
		globalPosition = &globalPaddlePositions.LeftPaddle
	} else {
		paddle = &RightPaddleData
		globalPosition = &globalPaddlePositions.RightPaddle
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
	gameState := map[string]float64{
		"leftPaddleData":  globalPaddlePositions.leftPaddle,
		"rightPaddleData": globalPaddlePositions.rightPaddle,
	}

	broadcast(gameState)
    //wsh.broadcastPaddlePositions()
}

func HandlePaddleCollision(
    paddle *Paddle,
    ball *ball.Ball,
    canvas *canvas.Canvas,
    globalPositions *PaddlePositions,
) {	
    ballRadius := ball.Radius

	leftPaddleRight := paddle.Width
	leftPaddleTop := float64(globalPaddlePositions.leftPaddle)
	leftPaddleBottom := leftPaddleTop + float64(paddle.Height)

	rightPaddleLeft := wsh.CanvasVar.Width - paddle.Width
	rightPaddleTop := float64(globalPaddlePositions.rightPaddle)
	rightPaddleBottom := rightPaddleTop + float64(paddle.Height)

	ballSpeed := math.Hypot(ball.Dx, ball.Dy)
	maxBounceAngle := math.Pi / 3 // 60 degrees max

	if ball.X-ballRadius <= leftPaddleRight &&
		ball.Y >= leftPaddleTop &&
		ball.Y <= leftPaddleBottom {
		//log.Println("collision with the left paddle detected, paddle height top and bottom", leftPaddleTop, leftPaddleBottom)

		relativePosition := (ball.Y - (leftPaddleTop + float64(paddle.Height)/2)) / (float64(paddle.Height) / 2)
		bounceAngle := relativePosition * maxBounceAngle
		ball.Dx = math.Abs(ballSpeed * math.Cos(bounceAngle))
		ball.Dy = ballSpeed * math.Sin(bounceAngle)
		ball.Dy += randomVariation()
		ball.X = leftPaddleRight + ballRadius
	}

	if ball.X+ballRadius >= rightPaddleLeft &&
		ball.Y >= rightPaddleTop &&
		ball.Y <= rightPaddleBottom {
		//log.Println("collision with the right paddle detected, paddle height top and bottom", rightPaddleTop, rightPaddleBottom)

		relativePosition := (ball.Y - (rightPaddleTop + float64(paddle.Height)/2)) / (float64(paddle.Height) / 2)
		bounceAngle := relativePosition * maxBounceAngle
		ball.Dx = -math.Abs(ballSpeed * math.Cos(bounceAngle))
		ball.Dy = ballSpeed * math.Sin(bounceAngle)
		ball.Dy += randomVariation()
		ball.X = rightPaddleLeft - ballRadius
	}
}
