import { type RoomJoinRequest, MsgType, type RoomCreateRequest, Message } from "./proto/gopong";

const createButton = document.getElementById("create-room") as HTMLButtonElement;
const statusDisplay = document.getElementById("status") as HTMLDivElement;
const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;

// Connect to the WebSocket server
let socket: Websocket;

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

function connectWebsocket() {
    socket = new WebSocket("ws://localhost:8080/ws");
    socket.binaryType = "arraybuffer";

    socket.onopen = async() => {
        console.log("Connected to websocket server");
        statusDisplay.textContent = "Connected to host";
    }

    socket.onmessage = async (event: MessageEvent): void => {
        const arrayBuffer = await event.data;
        const bytes = new Uint8Array(arrayBuffer);

        const message = Message.decode(bytes);
        handleMessage(message);
    }
}

function handleMessage (message: Message){
    switch (message.type) {

        case MsgType.room_create_response:
            const response = message.roomCreateResponse;
            console.log("Room Created Succesfully", response);
            
            if(response.roomId){
                roomCodeDisplay.textContent = `Room Code: ${response.roomId}`;
                statusDisplay.textContent = "Room Created Succesfully";

                setTimeout(() => {
                    window.location.href = `game.html?roomId=${encodeURIComponent(response.roomId)}&action=create`;
                }, 1000)
            } else {
                statusDisplay.textContent = "Failed To Create Room";
            }

            break;
    }
}

connectWebsocket();
