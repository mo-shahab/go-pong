// wsserver/handler.go

package wsserver

import (
	"log"
	"math/rand"
	"net/http"
	"time"
	"github.com/gorilla/websocket"
	"github.com/mo-shahab/go-pong/client"
	"github.com/mo-shahab/go-pong/game"
	pb "github.com/mo-shahab/go-pong/proto"
	"github.com/mo-shahab/go-pong/room"
	"google.golang.org/protobuf/proto"
)

// waiting room constants
const (
	MinPlayersToStart   = 2
	WaitingRoomDuration = 90
)

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	handler := &WebSocketHandler{
		Upgrader:     websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		Connections:  make(map[string]*client.Client),
		ConnToId:     make(map[*websocket.Conn]string),
		RoomManager:  room.NewRoomManager(),
		WaitingRooms: make(map[string]*room.WaitingRoomState),
	}
	
	// Create game engine with this handler as the event handler
	handler.GameEngine = game.NewEngine(handler)
	
	return handler
}

// GameEventHandler interface implementation
func (wsh *WebSocketHandler) OnBallPositionUpdate(ball *pb.Ball) {
	ballPositionMessage := &pb.BallPositionMessage{
		Ball: ball,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_ball_position,
		MessageType: &pb.Message_BallPosition{
			BallPosition: ballPositionMessage,
		},
	}

	message, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to encode ball position message: %v", err)
		return
	}

	wsh.broadcastToAll(message)
}

func (wsh *WebSocketHandler) OnScoreUpdate(leftScore, rightScore int32, whoScored string) {
	scoreUpdate := &pb.ScoreMessage{
		LeftScore:  leftScore,
		RightScore: rightScore,
		Scored:     whoScored,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_score,
		MessageType: &pb.Message_Score{
			Score: scoreUpdate,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal score message: %v", err)
		return
	}

	wsh.broadcastToAll(encoded)
}

func (wsh *WebSocketHandler) OnGameStateUpdate(leftPaddle, rightPaddle float64) {
	// This method can be used for additional game state updates if needed
	// Currently, paddle updates are handled in the movement message response
}

// Broadcast functions
func (wsh *WebSocketHandler) broadcastToAll(message []byte) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	for _, client := range wsh.Connections {
		select {
		case client.SendQueue <- message:
		default:
			log.Printf("Dropping message, send queue full for client %s", client.ID)
		}
	}
}

func (wsh *WebSocketHandler) broadcastToRoom(roomId string, message []byte) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	for _, client := range wsh.Connections {
		if client.RoomId == roomId {
			select {
			case client.SendQueue <- message:
			default:
				log.Printf("Dropping message, send queue full for client %s", client.ID)
			}
		}
	}
}

// getGameIdForClient returns the game ID for a client (uses room ID as game ID)
func (wsh *WebSocketHandler) getGameIdForClient(client *client.Client) string {
	if client.RoomId != "" {
		return client.RoomId // Room ID is the game ID
	}
	return "lobby" // Default lobby game for clients not in rooms
}

// disconnectPlayer handles player disconnection
func (wsh *WebSocketHandler) disconnectPlayer(conn *websocket.Conn) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	clientId, exists := wsh.ConnToId[conn]
	if !exists {
		return
	}

	client, exists := wsh.Connections[clientId]
	if !exists {
		return
	}

	gameId := wsh.getGameIdForClient(client)

	// Remove player from game engine
	if gameState, exists := wsh.GameEngine.GetGame(gameId); exists {
		gameState.RemovePlayer(client.Team)
	}

	// Remove from room if they're in one
	if client.RoomId != "" {
		wsh.RoomManager.RemoveClient(client.RoomId, clientId)
	}

	close(client.SendQueue)
	delete(wsh.Connections, clientId)
	delete(wsh.ConnToId, conn)

	conn.Close()
}

// assignTeam assigns a team to a client based on their room
func (wsh *WebSocketHandler) assignTeam(client *client.Client) {
	gameId := wsh.getGameIdForClient(client)
	
	// Count players in this specific game/room
	playersInGame := 0
	for _, c := range wsh.Connections {
		if wsh.getGameIdForClient(c) == gameId {
			playersInGame++
		}
	}

	if playersInGame < 2 {
		if playersInGame%2 == 0 {
			client.Team = "left"
		} else {
			client.Team = "right"
		}
	} else {
		// Random assignment for additional players
		if rand.Intn(100)%2 == 0 {
			client.Team = "left"
		} else {
			client.Team = "right"
		}
	}

	// Create game if it doesn't exist
	if _, exists := wsh.GameEngine.GetGame(gameId); !exists {
		wsh.GameEngine.CreateGame(gameId)
	}

	// Add player to game engine
	if gameState, exists := wsh.GameEngine.GetGame(gameId); exists {
		gameState.AddPlayer(client.Team)
	}

	log.Printf("Client %s assigned to team %s in game %s", client.ID, client.Team, gameId)
}

// ServeHTTP handles WebSocket connections
func (wsh *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error %s when connecting to the socket", err)
		return
	}

	clientId := conn.RemoteAddr().String() + "_" + time.Now().String()

	client := &client.Client{
		Conn:      conn,
		SendQueue: make(chan []byte, 100),
		ID:        clientId,
		RoomId:    "", // Start in lobby
	}

	// Assign team to client
	wsh.assignTeam(client)

	// Message queue goroutine
	go func() {
		for msg := range client.SendQueue {
			err := client.Conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Printf("Binary message write error: %v", err)
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
			log.Printf("Error reading message: %v", err)
			wsh.disconnectPlayer(conn)
			return
		}

		message := &pb.Message{}
		err = proto.Unmarshal(p, message)
		if err != nil {
			log.Printf("Error unmarshalling protobuf: %v", err)
			wsh.sendError(client, "Invalid protobuf format")
			continue
		}

		wsh.handleMessage(client, message)
	}
}

// handleMessage processes incoming messages
func (wsh *WebSocketHandler) handleMessage(client *client.Client, message *pb.Message) {
	switch message.Type {
	case pb.MsgType_room_create_request:
		wsh.handleRoomCreateRequest(client, message.GetRoomCreateRequest())
	
	case pb.MsgType_room_join_request:
		wsh.handleRoomJoinRequest(client, message.GetRoomJoinRequest())
	
	case pb.MsgType_init:
		wsh.handleInitMessage(client, message.GetInit())
	
	case pb.MsgType_movement:
		wsh.handleMovementMessage(client, message.GetMovement())
	
	default:
		log.Printf("Unknown message type: %v", message.Type)
	}
}

// handleInitMessage handles game initialization
func (wsh *WebSocketHandler) handleInitMessage(client *client.Client, init *pb.InitMessage) {
	gameId := wsh.getGameIdForClient(client)
	gameState, exists := wsh.GameEngine.GetGame(gameId)
	if !exists {
		log.Printf("Game state not found for game ID: %s", gameId)
		return
	}

	if !gameState.Initialized && init.Width > 0 && init.Height > 0 {
		gameState.Initialize(init.Width, init.Height, init.PaddleWidth, init.PaddleHeight)
		
		// Count players in this game
		playersInGame := 0
		for _, c := range wsh.Connections {
			if wsh.getGameIdForClient(c) == gameId {
				playersInGame++
			}
		}
		
		// Start ball updates if we have enough players
		if playersInGame > 1 && !gameState.BallRunning {
			gameState.StartBallUpdates(wsh.GameEngine, gameId)
		}
	}

	leftPaddle, rightPaddle, _, _ := gameState.GetGameState()

	// Count clients in this specific game/room
	clientsInGame := int32(0)
	for _, c := range wsh.Connections {
		if wsh.getGameIdForClient(c) == gameId {
			clientsInGame++
		}
	}

	initialGameState := &pb.InitialGameStateMessage{
		LeftPaddleData:  leftPaddle,
		RightPaddleData: rightPaddle,
		YourTeam:        client.Team,
		Clients:         clientsInGame,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_initial_game_state,
		MessageType: &pb.Message_InitialGameState{
			InitialGameState: initialGameState,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal initial game state: %v", err)
		return
	}

	client.SendQueue <- encoded
}

// handleMovementMessage handles paddle movement
func (wsh *WebSocketHandler) handleMovementMessage(client *client.Client, move *pb.MovementMessage) {
	gameId := wsh.getGameIdForClient(client)
	gameState, exists := wsh.GameEngine.GetGame(gameId)
	if !exists {
		return
	}

	leftPaddle, rightPaddle := gameState.MovePaddle(client.Team, move.Direction)
	
	// Count clients in this specific game/room
	clientsInGame := int32(0)
	for _, c := range wsh.Connections {
		if wsh.getGameIdForClient(c) == gameId {
			clientsInGame++
		}
	}

	gameStateMsg := &pb.GameStateMessage{
		LeftPaddleData:  &leftPaddle,
		RightPaddleData: &rightPaddle,
		YourTeam:        &client.Team,
		Clients:         &clientsInGame,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_game_state,
		MessageType: &pb.Message_GameState{
			GameState: gameStateMsg,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}

	// Broadcast to all clients in the same game/room
	if client.RoomId != "" {
		wsh.broadcastToRoom(client.RoomId, encoded)
	} else {
		client.SendQueue <- encoded // Just send to the client if they're in lobby
	}
}

// handleRoomCreateRequest handles room creation requests
func (wsh *WebSocketHandler) handleRoomCreateRequest(client *client.Client, req *pb.RoomCreateRequest) {

	roomId := wsh.RoomManager.CreateRoom(client, int(req.MaxPlayers))
	log.Println("This is the room Id that has been created: ", roomId)
	
	client.RoomId = roomId
	wsh.GameEngine.CreateGame(roomId)
	wsh.assignTeam(client)

	response := &pb.RoomCreateResponse{
		RoomId: roomId,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_room_create_response,
		MessageType: &pb.Message_RoomCreateResponse{
			RoomCreateResponse: response,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal room create response: %v", err)
		return
	}

	client.SendQueue <- encoded
	
	log.Printf("Room %s created by client %s", roomId, client.ID)
}

// handleRoomJoinRequest handles room join requests
func (wsh *WebSocketHandler) handleRoomJoinRequest(client *client.Client, req *pb.RoomJoinRequest) {
	success, errorMsg := wsh.RoomManager.JoinRoom(req.RoomId, client)
	
	if success {
		client.RoomId = req.RoomId
		
		// Reassign team for the room's game
		wsh.assignTeam(client)
		
		log.Printf("Client %s joined room %s", client.ID, req.RoomId)

	} else {
		wsh.sendError(client, errorMsg)
		log.Printf("Client %s failed to join room %s: %s", client.ID, req.RoomId, errorMsg)
	}
}

// sendError sends an error message to a client
func (wsh *WebSocketHandler) sendError(client *client.Client, errorMsg string) {
	errorMessage := &pb.ErrorMessage{
		Error: errorMsg,
	}

	wrappedMessage := &pb.Message{
		Type: pb.MsgType_error,
		MessageType: &pb.Message_Error{
			Error: errorMessage,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal error message: %v", err)
		return
	}

	client.SendQueue <- encoded
}
