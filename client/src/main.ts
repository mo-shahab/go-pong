import { type RoomJoinRequest, MsgType, type RoomCreateRequest, Message } from "./proto/gopong";

const createButton = document.getElementById("create-room") as HTMLButtonElement;
const joinButton = document.getElementById("join-room") as HTMLButtonElement;
const statusDisplay = document.getElementById("status") as HTMLDivElement;
const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;

// Connect to the WebSocket server
let socket: Websocket;

createButton.addEventListener("click", () => {
    const roomCreateRequest: RoomCreateRequest = { 
        maxPlayers: 10,
        clientId: localStorage.getItem("client-id"),
    };

    const wrappedMessagePlain = {
        type: MsgType.room_create_request,
        roomCreateRequest: roomCreateRequest, 
    };

    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    socket.send(encoded);
})

joinButton.addEventListener("click", () => {
    const roomCodeInput = document.getElementById("room-code-input") as HTMLInputElement;
    
    const roomCode: string = roomCodeInput.value;
    console.log("Clicked on the join room button, room code: ", roomCode);

    const roomJoinRequest: RoomJoinRequest = { 
        roomId: roomCode,
    };

    const wrappedMessagePlain = {
        type: MsgType.room_join_request,
        roomJoinRequest: roomJoinRequest,
    }

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

        case MsgType.init_client:
            console.log("Init client Message: ", message);
            const initClient = message.initClient;
            const clientId = initClient.clientId;
            localStorage.setItem("client-id", clientId);
            console.log("This is the clientId: ", clientId);
            break;

        case MsgType.room_create_response:
            const response = message.roomCreateResponse;
            console.log("Room Created Succesfully", response);
            
            if(response.roomId){
                roomCodeDisplay.textContent = `Room Code: ${response.roomId}`;
                statusDisplay.textContent = "Room Created Succesfully";

                // window.location.href = `game.html?roomId=${encodeURIComponent(response.roomId)}&action=create`;
            } else {
                statusDisplay.textContent = "Failed To Create Room";
            }

            break;


        case MsgType.room_join_response:
            console.log("The message for the join room has been received");
            break;
    }
}

connectWebsocket();
