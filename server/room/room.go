package room

import (
	"context"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mo-shahab/go-pong/client"
	"log"
	"sync"
)

// typedef to define the Room
type Room struct {
	ID         string
	Host       *websocket.Conn
	Clients    map[string]*client.Client
	MaxPlayers int
	Mu         sync.Mutex
}

// state of all the rooms
type RoomManager struct {
	Rooms map[string]*Room
	Mu    sync.Mutex
}

// waiting room status
type WaitingRoomState struct {
	Room *Room
	CurrentPlayers int
	TimeLeft int
	IsActive bool
	Ctx context.Context
	Cancel context.CancelFunc
	Mu sync.Mutex
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		Rooms: make(map[string]*Room),
	}
}

func NewWaitingRoomState(room *Room, timeLeft int, ctx context.Context, cancel context.CancelFunc) *WaitingRoomState {
	return &WaitingRoomState{
		Room:           room,
		CurrentPlayers: len(room.Clients),
		TimeLeft:       timeLeft,
		IsActive:       true,
		Ctx:            ctx,
		Cancel:         cancel,
	}
}


// helpers
func generateRoomId() string {
	return uuid.New().String()[:6]
}

// should return the room id
func (rm *RoomManager) CreateRoom(host *client.Client, maxPlayers int) string {
	rm.Mu.Lock()

	// should write this function probably
	roomId := generateRoomId()

	// this should basically create room with the structure above
	// so it will need an object of the type Room and stuff right ??

	room := &Room{
		ID:         roomId,
		Host:       host.Conn,
		Clients:    map[string]*client.Client{host.ID: host},
		MaxPlayers: maxPlayers,
	}

	rm.Rooms[roomId] = room
	log.Printf("Created Room with room id: %s, with host: %s", roomId, host.ID)

	rm.Mu.Unlock()

	return roomId
}

func (rm *RoomManager) JoinRoom(roomId string, client *client.Client) (bool, string) {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	room, exists := rm.Rooms[roomId]
	if !exists {
		return false, "Room id is invalid"
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()

	if len(room.Clients) >= room.MaxPlayers {
		return false, "Room is full"
	}

	room.Clients[client.ID] = client
	log.Println("Client %s joined the Room with room id: %s", client.ID, roomId)

	return true, ""
}

func (rm *RoomManager) RemoveClient(roomId string, clientId string) {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	room, exists := rm.Rooms[roomId]
	if !exists {
		return
	}

	room.Mu.Lock()
	defer room.Mu.Unlock()

	delete(room.Clients, clientId)

	if room.MaxPlayers == 0 || room.Clients[clientId].Conn == room.Host {

		for _, client := range room.Clients {
			// write the protobuf message saying that the room is closed (broadcast it basically)
			client.Conn.Close()
		}

		delete(rm.Rooms, roomId)
		log.Println("Room with %s has been closed", roomId)
	}
}

func (rm *RoomManager) GetRoom(roomId string) (*Room, bool) {
	rm.Mu.Lock()
	defer rm.Mu.Unlock()

	room, exists := rm.Rooms[roomId]

	return room, exists
}
