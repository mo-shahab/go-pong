package game

type PaddleData struct {
	MovementSum float64
	Velocity    float64
	Players     int
	Position    float64
}

type PaddlePositions struct {
	LeftPaddle  float64
	RightPaddle float64
}
