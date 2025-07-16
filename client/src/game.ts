import { Message, InitMessage, MovementMessage, MsgType } from "./proto/gopong";

// Connect to the WebSocket server
const socket = new WebSocket("ws://localhost:8080/ws");

// Get the canvas element and its 2D context
const canvas = document.getElementById("game-canvas") as HTMLCanvasElement;
const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;

const urlParams = new URLSearchParams(window.location.search);
const roomId = urlParams.get("roomId");

const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;
const statusDisplay = document.getElementById("status-display") as HTMLDivElement;

if (!roomId) {
    console.error("No roomId provided in URL");
    roomCodeDisplay.textContent = "Room Code: ERROR";
} else {
    roomCodeDisplay.textContent = `Room Code: ${roomId}`;
}

// Set canvas dimensions - use available space but with proper aspect ratio
const gameContainer = document.getElementById("game-container") as HTMLDivElement;
const canvasContainer = document.querySelector(".canvas-container") as HTMLDivElement;

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

let startTime: number = 0;
let timeLeft: number = 0;
let intervalId: number | undefined;

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

function startWaitTimer(): void {
    statusDisplay.textContent = `Waiting for players: ${timeLeft}s`;
    drawGame();
    intervalId = window.setInterval(() => {
        timeLeft--;
        statusDisplay.textContent = `Waiting for players: ${timeLeft}s`;
        drawGame();
        if (timeLeft <= 0) {
            clearInterval(intervalId);
            console.log("Timer expired, starting game or closing room");
            statusDisplay.textContent = "Starting game or closing room...";
        }
    }, 1000);
}

function startTimer(duration: number, message: string): void {
    isResetting = true;
    resetMessage = message;
    timeLeft = duration;
    startTime = Date.now();

    // Clear any existing interval
    if (intervalId) {
        clearInterval(intervalId);
    }

    // Set new interval
    intervalId = window.setInterval(() => {
        updateTimer();
    }, 1000);
}

function stopTimer(): void {
    if (intervalId) {
        clearInterval(intervalId);
        intervalId = undefined;
    }
    isResetting = false;
    resetMessage = '';
}

function updateTimer(): void {
    const elapsedSeconds = Math.floor((Date.now() - startTime) / 1000);
    timeLeft = Math.max(0, 3 - elapsedSeconds); 

    drawGame();

    if (timeLeft <= 0) {
        stopTimer();
    }
}

// WebSocket event handlers
socket.onopen = (): void => {
    console.log("Connected to WebSocket server");
    statusDisplay.textContent = "Connected";

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

    console.log("Plain init message: ", wrappedMessagePlain);

    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
    console.log("This is the encoded message from the client", encoded);
    console.log("Sending InitMessage (object representation):", initDataPlain);

    socket.send(encoded);
};

socket.onmessage = async (event: MessageEvent): void => {
    const arrayBuffer = await event.data.arrayBuffer();
    const bytes = new Uint8Array(arrayBuffer);
    const message = Message.decode(bytes);

    switch (message.type) {
        case MsgType.initial_game_state:
            const initial = message.initialGameState;
            console.log("Initial game state received", initial);
            if (timeLeft === 0) {
                timeLeft = 90;
                startWaitTimer();
            }
            leftPaddleY = initial.leftPaddleData ?? leftPaddleY;
            rightPaddleY = initial.rightPaddleData ?? rightPaddleY;
            break;

        case MsgType.game_state:
            console.log("Game state update received", message);
            const gameState = message.gameState;

            if (gameState.leftPaddleData !== undefined) {
                console.log("Message about the left paddle data: ", gameState.leftPaddleData);
                if (gameState.clients !== undefined && gameState.clients < 2) {
                    leftPaddleY = Math.max(
                        0,
                        Math.min(leftPaddleY + gameState.leftPaddleData, gameHeight - paddleHeight)
                    );
                } else {
                    leftPaddleY = gameState.leftPaddleData;
                }
            }

            if (gameState.rightPaddleData !== undefined) {
                if (gameState.clients !== undefined && gameState.clients < 2) {
                    rightPaddleY = Math.max(
                        0,
                        Math.min(rightPaddleY + gameState.rightPaddleData, gameHeight - paddleHeight)
                    );
                } else {
                    rightPaddleY = gameState.rightPaddleData;
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
