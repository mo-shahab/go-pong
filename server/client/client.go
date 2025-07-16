package client

import (
	"github.com/gorilla/websocket"
)

type Client struct {
	Conn      *websocket.Conn
	SendQueue chan []byte
	Team      string
	ID        string
	RoomId    string
}
