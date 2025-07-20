package wsserver

import (
	"log"
	"time"
	"context"
	pb "github.com/mo-shahab/go-pong/proto"
	"github.com/mo-shahab/go-pong/room"
	"github.com/mo-shahab/go-pong/client"
	"google.golang.org/protobuf/proto"
)

func (wsh *WebSocketHandler) startWaitingRoom(roomId string) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()
	
	roomObj, exists := wsh.RoomManager.GetRoom(roomId)
	if !exists {
		log.Println("The room does not exist")
		return
	}
	
	ctx, cancel := context.WithTimeout(
		context.Background(), WaitingRoomDuration*time.Second,
	)

	log.Println("Time left is set to ", WaitingRoomDuration)
	
	waitingRoom := room.NewWaitingRoomState(roomObj, WaitingRoomDuration, ctx, cancel)
	wsh.WaitingRooms[roomId] = waitingRoom
	go wsh.runWaitingRoom(waitingRoom)
	log.Println("Started waiting room for roomId: ", roomId)
}

func (wsh *WebSocketHandler) runWaitingRoom(waitingRoom *room.WaitingRoomState) {
    ticker := time.NewTicker(1000 * time.Millisecond) // For UI updates
    defer ticker.Stop()
    
    for {
        select {
        case <-waitingRoom.Ctx.Done():
            log.Printf("Waiting room %s timed out", waitingRoom.Room.ID)
            waitingRoom.Mu.Lock()
            waitingRoom.IsActive = false
            waitingRoom.Mu.Unlock()
            
            if waitingRoom.CurrentPlayers >= MinPlayersToStart {
                wsh.startGame(waitingRoom.Room.ID)
            } else {
                wsh.closeRoom(waitingRoom.Room.ID, "")
            }
            return
            
        case <-ticker.C:
            waitingRoom.Mu.Lock()
            
            if !waitingRoom.IsActive {
                waitingRoom.Mu.Unlock()
                return
            }
            
            // Calculate remaining time from context deadline
            deadline, ok := waitingRoom.Ctx.Deadline()
            if ok {
                waitingRoom.TimeLeft = int(time.Until(deadline).Seconds())
            }
            
            arePlayersFilled := waitingRoom.CurrentPlayers >= waitingRoom.Room.MaxPlayers
            areMinimumPlayers := waitingRoom.CurrentPlayers >= MinPlayersToStart
            
            if areMinimumPlayers && arePlayersFilled {
                log.Printf("Room %s has enough players, starting game immediately", waitingRoom.Room.ID)
                waitingRoom.IsActive = false
                waitingRoom.Mu.Unlock()
                wsh.startGame(waitingRoom.Room.ID)
                return
            }
            
            // Broadcast timer update to clients here
			wsh.broadcastWaitingRoomMessage(waitingRoom)
            waitingRoom.Mu.Unlock()
        }
    }
}

func (wsh *WebSocketHandler) broadcastWaitingRoomMessage(waitingRoom *room.WaitingRoomState) {

	roomMessage := &pb.Room {
		Id: waitingRoom.Room.ID,
		MaxPlayers: int32(waitingRoom.Room.MaxPlayers),
	}
	
	waitingRoomMessage := &pb.WaitingRoomStateMessage {
		Room: roomMessage,
		CurrentPlayers: int32(waitingRoom.CurrentPlayers),
		TimeLeft: int32(waitingRoom.TimeLeft),
		IsActive: waitingRoom.IsActive,
	}

	wrappedMessage := &pb.Message {
		Type: pb.MsgType_waiting_room_state,
		MessageType: &pb.Message_WaitingRoomState {
			WaitingRoomState: waitingRoomMessage,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)
	
	if err != nil {
		log.Println("Failed to marshal the waiting room message")
		return
	}

	wsh.broadcastToAll(encoded)
}

func (wsh *WebSocketHandler) addPlayerToWaitingRoom(roomId string, client *client.Client) bool {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	waitingRoom, exists := wsh.WaitingRooms[roomId]
	if !exists {
		log.Println("Waiting Room does not exist")
	}

	waitingRoom.Mu.Lock()
	defer waitingRoom.Mu.Unlock()

	if waitingRoom.CurrentPlayers >= waitingRoom.Room.MaxPlayers {
		return false
	}	

	waitingRoom.CurrentPlayers++

	client.RoomId = roomId

	log.Println("Player %s joined room %s. Current Players: %d/%d", 
		client.ID, 
		roomId, 
		waitingRoom.CurrentPlayers, 
		waitingRoom.Room.MaxPlayers,
		)

	return true
}

func (wsh *WebSocketHandler) removePlayerFromRoom (roomId string, client client.Client) bool {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	waitingRoom, exists := wsh.WaitingRooms[roomId]
	if !exists {
		log.Println("Waiting Room does not exist")
	}
	
	waitingRoom.Mu.Lock()
	defer waitingRoom.Mu.Unlock()

	if waitingRoom.CurrentPlayers <= 0 {
		return false
	}

	waitingRoom.CurrentPlayers--
	waitingRoom.IsActive = false
	waitingRoom.Cancel()
	delete(wsh.WaitingRooms, roomId)

	log.Println("Removed the client from the waiting room")
	return true
}

func (wsh *WebSocketHandler) startGame (roomId string) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()

	waitingRoom, exists := wsh.WaitingRooms[roomId]
	if exists {
		waitingRoom.Cancel()
		delete(wsh.WaitingRooms, roomId)
	}

	gameStartMessage := &pb.GameStartMessage {
		RoomId: waitingRoom.Room.ID,
	}

	wrappedMessage := &pb.Message {
		Type: pb.MsgType_game_start,
		MessageType: &pb.Message_GameStart {
			GameStart: gameStartMessage,
		},
	}

	encoded, err := proto.Marshal(wrappedMessage)

	if err != nil {
		log.Println("Error occured while marshaling: ", err)
	}

	wsh.broadcastToRoom(roomId, encoded)
}

func (wsh *WebSocketHandler) closeRoom(roomId string, reason string) {
	wsh.Mu.Lock()
	defer wsh.Mu.Unlock()
	
	if waitingRoom, exists := wsh.WaitingRooms[roomId]; exists {
		waitingRoom.Cancel()
		delete(wsh.WaitingRooms, roomId)
	}
	
	// Send room closed message
	roomClosedMessage := &pb.RoomClosedMessage{
		RoomId: roomId,
		Reason: reason,
	}
	
	wrappedMessage := &pb.Message{
		Type: pb.MsgType_room_closed,
		MessageType: &pb.Message_RoomClosed{
			RoomClosed: roomClosedMessage,
		},
	}
	
	encoded, err := proto.Marshal(wrappedMessage)
	if err != nil {
		log.Printf("Failed to marshal room closed message: %v", err)
		return
	}
	
	wsh.broadcastToRoom(roomId, encoded)
	log.Printf("Room %s closed: %s", roomId, reason)
}


