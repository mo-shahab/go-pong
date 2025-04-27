package client

import (
    "github.com/gorilla/websocket"
)

type Client struct {
	conn      *websocket.Conn
	sendQueue chan interface{}
	team      string
	id        string
}
