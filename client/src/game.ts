import { Message, InitMessage, MovementMessage, MsgType } from "./proto/gopong";

// Connect to the WebSocket server
const socket = new WebSocket("ws://localhost:8080/ws");

// Get the canvas element and its 2D context
const canvas = document.getElementById("game-canvas") as HTMLCanvasElement;
const ctx = canvas.getContext("2d") as CanvasRenderingContext2D;

// Set canvas dimensions
canvas.width = window.innerWidth * 0.9;
canvas.height = window.innerHeight * 0.9;

// Game dimensions and settings
const gameWidth: number = canvas.width;
const gameHeight: number = canvas.height;
const paddleWidth: number = gameWidth * 0.01;
const paddleHeight: number = gameHeight * 0.15;

// Game state variables
let leftPaddleY: number | undefined;
let rightPaddleY: number | undefined;
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


console.log("Fix that error and print scored: ", scored);

// Draw the game elements
function drawGame(): void {
    // Clear the canvas
    ctx.clearRect(0, 0, gameWidth, gameHeight);

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

    // 1. Create a plain JavaScript object matching the InitMessage structure
    const initDataPlain: InitMessage = { // This is a plain object, not an instance of a class
        width: gameWidth,
        height: gameHeight,
        paddleHeight: paddleHeight,
        paddleWidth: paddleWidth,
    };

    // 2. Wrap the plain InitMessage object inside a plain Message object for the oneof
    const wrappedMessagePlain = {
        type: MsgType.init,
        init: initDataPlain, // Assign the plain initData object to the 'init' field of the oneof
    };

    console.log("Plain init message: ", wrappedMessagePlain);

    // 3. Use the static .encode() method on the Message type with the plain object
    //    and then .finish() to get the Uint8Array.
    const encoded: Uint8Array = Message.encode(wrappedMessagePlain).finish();

    console.log("This is the encoded message from the client", encoded);
    console.log("Sending InitMessage (object representation):", initDataPlain);

    // 4. Send the binary data over the WebSocket
    //    Do NOT JSON.stringify it. Send the Uint8Array directly.
    socket.send(encoded);
};

socket.onmessage = async (event: MessageEvent): void => {
    // const data: GameData = JSON.parse(event.data);

    const arrayBuffer = await event.data.arrayBuffer();
    const bytes = new Uint8Array(arrayBuffer);

    const message = Message.decode(bytes);

    switch (message.type) {
        case MsgType.initial_game_state:
            const initial = message.initialGameState;
            console.log("Initial game state received", initial);
            leftPaddleY = initial.leftPaddleData ?? leftPaddleY;
            rightPaddleY = initial.rightPaddleData ?? rightPaddleY;
            // yourTeam = message.yourTeam; // if needed for controls
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

            // Start timer with appropriate message
            let scoringTeam = "Team";
            if (score.scored) {
                scoringTeam = score.scored === 'left' ? "Left Team" : "Right Team";
                startTimer(3, `${scoringTeam} scored! Board will reset soon.`);
            }

            break;

        case MsgType.error:
            const error = message.error;
            console.log("Error received from the server: ", error);
            // yourTeam = message.yourTeam; // if needed for controls
            break;

        default:
            break;
    }

    // Redraw the game with updated positions
    drawGame();
};

socket.onclose = (): void => {
    console.log("Disconnected from WebSocket server");
};

socket.onerror = (error: Event): void => {
    console.error("WebSocket error:", error);
};

// Keyboard event handler
document.addEventListener("keydown", (e: KeyboardEvent): void => {
    let movement: MovementMessage | null = null;
    let encoded: Uint8Array | null = null;

    if (e.key === "w") {

        console.log("Button pressed");
        movement = { direction: "up", paddle: "left" };
        const wrappedMessagePlain = {
            type: MsgType.movement,
            movement: movement,
        };
        encoded = Message.encode(wrappedMessagePlain).finish();

    } else if (e.key === "s") {

        movement = { direction: "down", paddle: "left" };
        const wrappedMessagePlain = {
            type: MsgType.movement,
            movement: movement, 
        };
        encoded = Message.encode(wrappedMessagePlain).finish();

    }

    if (movement) {
        socket.send(encoded);
    }
});

// Clean up WebSocket connection when page closes
window.addEventListener('beforeunload', (_: BeforeUnloadEvent): void => {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, "Page is closing");
    }
});
