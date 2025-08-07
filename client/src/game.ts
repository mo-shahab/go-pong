// main.ts
import { type RoomJoinRequest, MsgType, type RoomCreateRequest, Message } from "./proto/gopong";
import { wsManager } from "./websocket-manager";
import { initGameView } from "./game";

const createButton = document.getElementById("create-room") as HTMLButtonElement;
const joinButton = document.getElementById("join-room") as HTMLButtonElement;
const statusDisplay = document.getElementById("status") as HTMLDivElement;
const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;

function handleMessage(message: Message) {
    switch (message.type) {
        case MsgType.init_client:
            console.log("Init client Message: ", message);
            const initClient = message.initClient;
            if (initClient && initClient.clientId) {
                localStorage.setItem("client-id", initClient.clientId);
                console.log("This is the clientId: ", initClient.clientId);
                statusDisplay.textContent = "Connected and authenticated";
            }
            break;
            
        case MsgType.room_create_response:
            const response = message.roomCreateResponse;
            console.log("Room Created Successfully", response);
            
            if (response && response.roomId) {
                roomCodeDisplay.textContent = `Room Code: ${response.roomId}`;
                statusDisplay.textContent = "Room Created Successfully";
                // After join/create success:
                initGameView(roomId, "join");
            } else {
                statusDisplay.textContent = "Failed To Create Room";
            }
            break;
            
        case MsgType.room_join_response:
            console.log("Join room response received", message);
            const joinResponse = message.roomJoinResponse;
            
            if (joinResponse && joinResponse.success && joinResponse.roomId) {
                statusDisplay.textContent = "Joined room successfully";
                setTimeout(() => {
                    initGameView(roomId, "join");
                }, 100);
            } else {
                statusDisplay.textContent = "Failed to join room";
            }
            break;
            
        default:
            console.log("Unhandled message type:", message.type);
    }
}

// Register the message handler immediately
wsManager.addMessageHandler(handleMessage);

// Set initial status
statusDisplay.textContent = "Connecting to server...";

// Wait for connection to be ready
wsManager.connect()
    .then(() => {
        console.log("WebSocket connected successfully");
        statusDisplay.textContent = "Connected to server";
    })
    .catch((error) => {
        console.error("Failed to connect:", error);
        statusDisplay.textContent = "Connection failed - Check server";
    });

createButton.addEventListener("click", async () => {
    if (!wsManager.isConnected()) {
        statusDisplay.textContent = "Reconnecting...";
        try {
            await wsManager.connect();
        } catch (error) {
            statusDisplay.textContent = "Connection failed";
            return;
        }
    }

    const clientId = localStorage.getItem("client-id");
    if (!clientId) {
        statusDisplay.textContent = "Waiting for authentication...";
        return;
    }

    statusDisplay.textContent = "Creating room...";
    
    const roomCreateRequest: RoomCreateRequest = { 
        maxPlayers: 10,
        clientId: clientId,
    };
    
    const wrappedMessage = {
        type: MsgType.room_create_request,
        roomCreateRequest: roomCreateRequest, 
    };
    
    const encoded: Uint8Array = Message.encode(wrappedMessage).finish();
    wsManager.send(encoded);
});

joinButton.addEventListener("click", async () => {
    if (!wsManager.isConnected()) {
        statusDisplay.textContent = "Reconnecting...";
        try {
            await wsManager.connect();
        } catch (error) {
            statusDisplay.textContent = "Connection failed";
            return;
        }
    }

    const clientId = localStorage.getItem("client-id");
    if (!clientId) {
        statusDisplay.textContent = "Waiting for authentication...";
        return;
    }

    const roomCodeInput = document.getElementById("room-code-input") as HTMLInputElement;
    const roomCode = roomCodeInput.value.trim();
    
    if (!roomCode) {
        statusDisplay.textContent = "Please enter a room code";
        roomCodeInput.focus();
        return;
    }
    
    statusDisplay.textContent = "Joining room...";
    
    const roomJoinRequest: RoomJoinRequest = { 
        roomId: roomCode,
        clientId: clientId,
    };
    
    const wrappedMessage = {
        type: MsgType.room_join_request,
        roomJoinRequest: roomJoinRequest,
    };
    
    const encoded: Uint8Array = Message.encode(wrappedMessage).finish();
    wsManager.send(encoded);
});

// Clean up when leaving the page
window.addEventListener('beforeunload', () => {
    wsManager.removeMessageHandler(handleMessage);
});



