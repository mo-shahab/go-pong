package ball

type Ball struct {
	X, Y    float64
	Dx, Dy  float64
	Radius  float64
	Visible bool
}

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
