import { type RoomJoinRequest, MsgType, type RoomCreateRequest, Message } from "./proto/gopong";

const createButton = document.getElementById("create-room") as HTMLButtonElement;

// Connect to the WebSocket server
const socket = new WebSocket("ws://localhost:8080/ws");

createButton.addEventListener("click", () => {
    const roomCreateRequest: RoomCreateRequest = { 
        maxPlayers: 10,
    };

    const wrappedMessagePlain = {
        type: MsgType.room_create_request,
        room_create_request: roomCreateRequest, 
    };

    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    socket.send(encoded);
})

