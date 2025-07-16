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
        roomCreateRequest: roomCreateRequest, 
    };

    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    socket.send(encoded);
})

socket.onmessage = async (event: MessageEvent): void => {
    const arrayBuffer = await event.data.arrayBuffer();
    const bytes = new Uint8Array(arrayBuffer);

    const message = Message.decode(bytes);
    switch (message.type) {

        case MsgType.room_create_response:
            const response = message.roomCreateResponse;
            console.log("Room Created Succesfully", response);
            break;
    }
}
