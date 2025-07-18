import { Message, InitMessage, MovementMessage, MsgType } from "./proto/gopong";

// Connect to the WebSocket server
const socket = new WebSocket("ws://localhost:8080/ws");

// Get the canvas element and its 2D context
const canvas = document.getElementById("game-canvas") as HTMLCanvasElement;
const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;

const urlParams = new URLSearchParams(window.location.search);
const roomId = urlParams.get("roomId");
const action = urlParams.get("action");

const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;
const statusDisplay = document.getElementById("status-display") as HTMLDivElement;
const playerCountDisplay = document.getElementById("player-count-display") as HTMLDivElement;

let roomState = {
    isActive: false,
    gameStarted: false,
    currentRoomId: "",
    currentPlayers: 0, // players in the room -> its same as in the server
    maxPlayers: 0,
    timeLeft: 0,
}

if (!roomId && action !== "create") {
    console.error("No roomId provided in URL");
    roomCodeDisplay.textContent = "Room Code: ERROR";
    statusDisplay.textContent = "Invalid Room Configuration";
} else {
    roomCodeDisplay.textContent = `Room Code: ${roomId}`;
    statusDisplay.textContent = "Connecting...";
}

// Set canvas dimensions - use available space but with proper aspect ratio
const gameContainer = document.getElementById("game-container") as HTMLDivElement;
const canvasContainer = document.querySelector(".canvas-container") as HTMLDivElement;

canvas.style.display = "none";

// Calculate canvas size based on the canvas container, not the full game container
const containerRect = canvasContainer.getBoundingClientRect();

// Account for the player list panels and padding
const maxWidth = Math.min(window.innerWidth * 0.8, window.innerHeight * 0.7 * (16/9)); // 80vw or height-based
const maxHeight = window.innerHeight * 0.7; // 70vh

// Maintain 4:3 aspect ratio (better for gameplay)
const aspectRatio = 16 / 9;
let canvasWidth = maxWidth;
let canvasHeight = canvasWidth / aspectRatio;

if (canvasHeight > maxHeight) {
    canvasHeight = maxHeight;
    canvasWidth = canvasHeight * aspectRatio;
}

canvas.width = canvasWidth;
canvas.height = canvasHeight;

// Game dimensions and settings
const gameWidth: number = canvasWidth;
const gameHeight: number = canvasHeight;
const paddleWidth: number = 12; // Fixed paddle width
const paddleHeight: number = gameHeight * 0.12; // Slightly smaller paddle height

// Game state variables
let leftPaddleY: number = gameHeight / 2 - paddleHeight / 2;
let rightPaddleY: number = gameHeight / 2 - paddleHeight / 2;
let ballX: number = gameWidth / 2;
let ballY: number = gameHeight / 2;

let leftScore: number = 0;
let rightScore: number = 0;
let scored: string = '';

let isResetting: boolean = false;
let resetMessage: string = '';

console.log("Canvas dimensions:", { width: gameWidth, height: gameHeight });
console.log("Fix that error and print scored: ", scored);

// Draw the game elements
function drawGame(): void {
    // Clear the canvas
    ctx.clearRect(0, 0, gameWidth, gameHeight);

        ctx.fillStyle = "#ffffff";
    ctx.strokeStyle = "#ffffff";

    // Draw center line
    ctx.setLineDash([15, 15]);
    ctx.beginPath();
    ctx.moveTo(gameWidth / 2, 0);
    ctx.lineTo(gameWidth / 2, gameHeight);
    ctx.lineWidth = 2;
    ctx.stroke();
    ctx.setLineDash([]); // Reset line dash

    // Draw left paddle
    ctx.fillStyle = "white";
    ctx.fillRect(10, leftPaddleY || 0, paddleWidth, paddleHeight);

    // Draw right paddle
    ctx.fillRect(gameWidth - 20, rightPaddleY || 0, paddleWidth, paddleHeight);

    // Draw the ball
    ctx.beginPath();
    ctx.arc(ballX, ballY, 8, 0, Math.PI * 2);
    ctx.fill();

    ctx.font = '36px Arial';
    ctx.textAlign = 'center';
    ctx.fillText(leftScore.toString(), gameWidth / 4, 50);
    ctx.fillText(rightScore.toString(), 3 * gameWidth / 4, 50);

    if (isResetting && timeLeft > 0) {
        ctx.font = '24px Arial';
        ctx.fillStyle = "red";
        ctx.fillText(resetMessage, gameWidth / 2, gameHeight / 2 - 30);
        
        // Draw timer
        ctx.fillText(`Resetting in ${timeLeft} seconds`, gameWidth / 2, gameHeight / 2);
    }
}
// Initial draw
drawGame();

function updateWaitingRoomDisplay(
    currentPlayers: number, 
    maxPlayers: number, 
    timeLeft: number
): void {
    statusDisplay.textContent = `Waiting for players...(${timeLeft}s remaining)`
    playerCountDisplay.textContent = `Players: ${currentPlayers} / ${maxPlayers}`
    
    // hide canvas during waiting room
    canvas.style.display = "none";

    gameContainer.style.display = "flex";
    gameContainer.style.flexDirection = "column";
    gameContainer.style.alignItems = "center";
    gameContainer.style.justifyContent = "center";
}

function startGame(): void {
    gameState.gameStarted = true;
    gameState.inWaitingRoom = false;
    
    statusDisplay.textContent = "Game Started!";
    canvas.style.display = "block";
    
    // Send init message to server
    const initDataPlain: InitMessage = {
        width: gameWidth,
        height: gameHeight,
        paddleHeight: paddleHeight,
        paddleWidth: paddleWidth,
    };

    const wrappedMessagePlain = {
        type: MsgType.init,
        init: initDataPlain,
    };

    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    socket.send(encoded);
    
    // Start drawing
    drawGame();
}

function handleRoomClosed(reason: string): void {
    statusDisplay.textContent = `Room closed: ${reason}`;
    canvas.style.display = "none";
    
    // Optionally redirect to home page or show retry option
    setTimeout(() => {
        window.location.href = "/";
    }, 3000);
}

// WebSocket event handlers
socket.onopen = (): void => {
    console.log("Connected to WebSocket server");
    statusDisplay.textContent = "Connected";
    console.log("this is the action", action)

    // const initDataPlain: InitMessage = {
    //     width: gameWidth,
    //     height: gameHeight,
    //     paddleHeight: paddleHeight,
    //     paddleWidth: paddleWidth,
    // };
    //
    // const wrappedMessagePlain = {
    //     type: MsgType.init,
    //     init: initDataPlain,
    // };
    //
    // const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    // socket.send(encoded);
};



socket.onmessage = async (event: MessageEvent): void => {
    const arrayBuffer = await event.data.arrayBuffer();
    const bytes = new Uint8Array(arrayBuffer);
    const message = Message.decode(bytes);
    console.log(message);

    switch (message.type) {
        case MsgType.waiting_room_state:
            const waitingState = message.waitingRoomState;
            
            roomState.isActive = waitingState.isActive;
            roomState.currentPlayers = waitingState.currentPlayers;
            roomState.maxPlayers = waitingState.room.maxPlayers;
            roomState.timeLeft = waitingState.timeLeft;

            updateWaitingRoomDisplay(
                roomState.currentPlayers,
                roomState.maxPlayers,
                roomState.timeLeft
            );

            console.log("Waiting Room State message recieved");

            break;

        case MsgType.initial_game_state:
            const initial = message.initialGameState;
            console.log("Initial game state received", initial);
            leftPaddleY = initial.leftPaddleData ?? leftPaddleY;
            rightPaddleY = initial.rightPaddleData ?? rightPaddleY;
            break;

        case MsgType.game_state:
            console.log("Game state update received", message);
            const gameStateMsg = message.gameStateMsg;

            if (gameStateMsg.leftPaddleData !== undefined) {
                console.log("Message about the left paddle data: ", gameStateMsg.leftPaddleData);
                if (gameStateMsg.clients !== undefined && gameStateMsg.clients < 2) {
                    leftPaddleY = Math.max(
                        0,
                        Math.min(leftPaddleY + gameStateMsg.leftPaddleData, gameHeight - paddleHeight)
                    );
                } else {
                    leftPaddleY = gameStateMsg.leftPaddleData;
                }
            }

            if (gameStateMsg.rightPaddleData !== undefined) {
                if (gameStateMsg.clients !== undefined && gameStateMsg.clients < 2) {
                    rightPaddleY = Math.max(
                        0,
                        Math.min(rightPaddleY + gameStateMsg.rightPaddleData, gameHeight - paddleHeight)
                    );
                } else {
                    rightPaddleY = gameStateMsg.rightPaddleData;
                }
            }
            break;

        case MsgType.ball_position:
            const ballPos = message.ballPosition;
            ballX = ballPos.ball.x;
            ballY = ballPos.ball.y;
            break;

        case MsgType.score:
            console.log("Scored!");
            const score = message.score;
            console.log("Score Message: ", score);
            leftScore = score.leftScore || 0;
            rightScore = score.rightScore || 0;
            scored = score.scored || '';

            let scoringTeam = "Team";
            if (score.scored) {
                scoringTeam = score.scored === 'left' ? "Left Team" : "Right Team";
                startTimer(3, `${scoringTeam} scored! Board will reset soon.`);
            }
            break;

        case MsgType.error:
            const error = message.error;
            console.log("Error received from the server: ", error);
            statusDisplay.textContent = `Error: ${error}`;
            break;

        default:
            break;
    }

    drawGame();
};

socket.onclose = (): void => {
    console.log("Disconnected from WebSocket server");
    statusDisplay.textContent = "Disconnected";
};

socket.onerror = (error: Event): void => {
    console.error("WebSocket error:", error);
    statusDisplay.textContent = "Connection Error";
};

// Keyboard event handler
document.addEventListener("keydown", (e: KeyboardEvent): void => {
    let movement: MovementMessage | null = null;
    let encoded: Uint8Array | null = null;

    if (e.key === "w") {
        console.log("W button pressed");
        movement = { direction: "up", paddle: "left" };
        const wrappedMessagePlain = {
            type: MsgType.movement,
            movement: movement,
        };
        encoded = Message.encode(wrappedMessagePlain).finish();
    } else if (e.key === "s") {
        console.log("S button pressed");
        movement = { direction: "down", paddle: "left" };
        const wrappedMessagePlain = {
            type: MsgType.movement,
            movement: movement, 
        };
        encoded = Message.encode(wrappedMessagePlain).finish();
    }

    if (movement && encoded) {
        socket.send(encoded);
    }
});

// Handle window resize
window.addEventListener('resize', () => {
    // Recalculate canvas size using viewport units with 16:9 ratio
    const maxWidth = Math.min(window.innerWidth * 0.8, window.innerHeight * 0.7 * (16/9));
    const maxHeight = window.innerHeight * 0.7;
    
    let newCanvasWidth = maxWidth;
    let newCanvasHeight = newCanvasWidth / aspectRatio;
    
    if (newCanvasHeight > maxHeight) {
        newCanvasHeight = maxHeight;
        newCanvasWidth = newCanvasHeight * aspectRatio;
    }
    
    canvas.width = newCanvasWidth;
    canvas.height = newCanvasHeight;
    
    // Update game dimensions and paddle positions
    const newGameWidth = newCanvasWidth;
    const newGameHeight = newCanvasHeight;
    const newPaddleHeight = newGameHeight * 0.12;
    
    // Adjust paddle positions to stay within bounds
    leftPaddleY = Math.max(0, Math.min(leftPaddleY, newGameHeight - newPaddleHeight));
    rightPaddleY = Math.max(0, Math.min(rightPaddleY, newGameHeight - newPaddleHeight));
    
    drawGame();
});
// Clean up WebSocket connection when page closes
window.addEventListener('beforeunload', (_: BeforeUnloadEvent): void => {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, "Page is closing");
    }
});
