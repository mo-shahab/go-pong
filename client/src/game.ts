import { Message, InitMessage, MovementMessage, MsgType } from "./proto/gopong";
import { wsManager } from "./websocket-manager";
import { gameState } from "./game-state";

console.log("this is room id", gameState.getRoomId());

// Get DOM elements
const canvas = document.getElementById("game-canvas") as HTMLCanvasElement;
const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;
const gameContainer = document.getElementById("game-container") as HTMLDivElement;
const canvasContainer = document.querySelector(".canvas-container") as HTMLDivElement;
const roomCodeDisplay = document.getElementById("room-code-display") as HTMLDivElement;
const statusDisplay = document.getElementById("status-display") as HTMLDivElement;
const playerCountDisplay = document.getElementById("player-count-display") as HTMLDivElement;

// Room state
let roomState = {
    isActive: false,
    gameStarted: false,
    currentRoomId: "",
    currentPlayers: 0,
    maxPlayers: 0,
    timeLeft: 0,
};

// Game dimensions and settings
let gameWidth: number;
let gameHeight: number;
const paddleWidth: number = 12;
const paddleHeight: number = gameHeight * 0.12; // Will be updated after canvas sizing

// Game state variables
let leftPaddleY: number = 0;
let rightPaddleY: number = 0;
let ballX: number = 0;
let ballY: number = 0;
let leftScore: number = 0;
let rightScore: number = 0;
let scored: string = '';
let isResetting: boolean = false;
let resetMessage: string = '';

// Function to set up canvas dimensions
function setupCanvas(): void {
    const containerRect = canvasContainer.getBoundingClientRect();
    const maxWidth = Math.min(window.innerWidth * 0.8, window.innerHeight * 0.7 * (16/9));
    const maxHeight = window.innerHeight * 0.7;
    const aspectRatio = 16 / 9;

    gameWidth = maxWidth;
    gameHeight = gameWidth / aspectRatio;

    if (gameHeight > maxHeight) {
        gameHeight = maxHeight;
        gameWidth = gameHeight * aspectRatio;
    }

    canvas.width = gameWidth;
    canvas.height = gameHeight;

    // Initialize paddle and ball positions
    leftPaddleY = gameHeight / 2 - paddleHeight / 2;
    rightPaddleY = gameHeight / 2 - paddleHeight / 2;
    ballX = gameWidth / 2;
    ballY = gameHeight / 2;

    console.log("Canvas dimensions:", { width: gameWidth, height: gameHeight });
}

// Draw the game elements
function drawGame(): void {
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
    ctx.setLineDash([]);

    // Draw left paddle
    ctx.fillRect(10, leftPaddleY || 0, paddleWidth, paddleHeight);

    // Draw right paddle
    ctx.fillRect(gameWidth - 20, rightPaddleY || 0, paddleWidth, paddleHeight);

    // Draw the ball
    ctx.beginPath();
    ctx.arc(ballX, ballY, 8, 0, Math.PI * 2);
    ctx.fill();

    // Draw scores
    ctx.font = '36px Arial';
    ctx.textAlign = 'center';
    ctx.fillText(leftScore.toString(), gameWidth / 4, 50);
    ctx.fillText(rightScore.toString(), 3 * gameWidth / 4, 50);

    if (isResetting && roomState.timeLeft > 0) {
        ctx.font = '24px Arial';
        ctx.fillStyle = "red";
        ctx.fillText(resetMessage, gameWidth / 2, gameHeight / 2 - 30);
        ctx.fillText(`Resetting in ${roomState.timeLeft} seconds`, gameWidth / 2, gameHeight / 2);
    }
}

// Update waiting room display
function updateWaitingRoomDisplay(currentPlayers: number, maxPlayers: number, timeLeft: number): void {
    statusDisplay.textContent = `Waiting for players... (${timeLeft}s remaining)`;
    playerCountDisplay.textContent = `Players: ${currentPlayers} / ${maxPlayers}`;
    canvas.style.display = "none";
    gameContainer.style.display = "flex";
    gameContainer.style.flexDirection = "column";
    gameContainer.style.alignItems = "center";
    gameContainer.style.justifyContent = "center";
}

// Start the game
function startGame(): void {
    roomState.gameStarted = true;
    statusDisplay.textContent = "Game Started!";
    canvas.style.display = "block";

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
    wsManager.send(encoded);

    drawGame();
}

// Handle room closed
function handleRoomClosed(reason: string): void {
    statusDisplay.textContent = `Room closed: ${reason}`;
    canvas.style.display = "none";
    setTimeout(() => {
        window.location.href = "/";
    }, 3000);
}

// WebSocket message handler
function handleGameMessage(message: Message): void {
    switch (message.type) {
        case MsgType.waiting_room_state:
            const waitingState = message.waitingRoomState;
            roomState.isActive = waitingState.isActive;
            roomState.currentPlayers = waitingState.currentPlayers;
            roomState.maxPlayers = waitingState.room.maxPlayers;
            roomState.timeLeft = waitingState.timeLeft;
            updateWaitingRoomDisplay(roomState.currentPlayers, roomState.maxPlayers, roomState.timeLeft);
            break;

        case MsgType.initial_game_state:
            const initial = message.initialGameState;
            console.log("Initial game state received", initial);
            leftPaddleY = initial.leftPaddleData ?? leftPaddleY;
            rightPaddleY = initial.rightPaddleData ?? rightPaddleY;
            startGame();
            break;

        case MsgType.game_state:
            const gameStateMsg = message.gameStateMsg;
            if (gameStateMsg.leftPaddleData !== undefined) {
                leftPaddleY = gameStateMsg.clients < 2
                    ? Math.max(0, Math.min(leftPaddleY + gameStateMsg.leftPaddleData, gameHeight - paddleHeight))
                    : gameStateMsg.leftPaddleData;
            }
            if (gameStateMsg.rightPaddleData !== undefined) {
                rightPaddleY = gameStateMsg.clients < 2
                    ? Math.max(0, Math.min(rightPaddleY + gameStateMsg.rightPaddleData, gameHeight - paddleHeight))
                    : gameStateMsg.rightPaddleData;
            }
            drawGame();
            break;

        case MsgType.ball_position:
            const ballPos = message.ballPosition;
            ballX = ballPos.ball.x;
            ballY = ballPos.ball.y;
            drawGame();
            break;

        case MsgType.score:
            const score = message.score;
            leftScore = score.leftScore || 0;
            rightScore = score.rightScore || 0;
            scored = score.scored || '';
            if (score.scored) {
                const scoringTeam = score.scored === 'left' ? "Left Team" : "Right Team";
                isResetting = true;
                resetMessage = `${scoringTeam} scored! Board will reset soon.`;
                roomState.timeLeft = 3; // Assuming server sends reset time
            }
            drawGame();
            break;

        case MsgType.error:
            const error = message.error;
            statusDisplay.textContent = `Error: ${error}`;
            handleRoomClosed(error);
            break;

        default:
            console.log("Unhandled message type in game:", message.type);
    }
}

// Register message handler
wsManager.addMessageHandler(handleGameMessage);

// Keyboard event handler for paddle movement
document.addEventListener("keydown", (e: KeyboardEvent): void => {
    let movement: MovementMessage | null = null;
    if (e.key === "w") {
        movement = { direction: "up", paddle: "left" };
    } else if (e.key === "s") {
        movement = { direction: "down", paddle: "left" };
    }

    if (movement) {
        const wrappedMessagePlain = {
            type: MsgType.movement,
            movement: movement,
        };
        const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();
        wsManager.send(encoded);
    }
});

// Handle window resize
window.addEventListener('resize', () => {
    setupCanvas();
    leftPaddleY = Math.max(0, Math.min(leftPaddleY, gameHeight - paddleHeight));
    rightPaddleY = Math.max(0, Math.min(rightPaddleY, gameHeight - paddleHeight));
    drawGame();
});

// Clean up on page unload
window.addEventListener('beforeunload', () => {
    wsManager.removeMessageHandler(handleGameMessage);
});

// Initialize canvas
setupCanvas();
canvas.style.display = "none"; // Hide canvas initially
gameContainer.style.display = "none"; // Hide game view initially
