interface GameData {
    leftPaddleData?: number;
    rightPaddleData?: number;
    clients?: number;
    ball?: {
        x: number;
        y: number;
        radius?: number;
    };
    type?: string;
    leftScore?: number;
    rightScore?: number;
    scored?: string;
}

interface MovementData {
    type: string;
    direction: "up" | "down";
    paddle: "left" | "right";
}

interface InitData {
    type: string;
    width: number;
    height: number;
    paddleHeight: number;
    paddleWidth: number;
}

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

    const initData: InitData = {
        type: "init",
        width: gameWidth,
        height: gameHeight,
        paddleHeight: paddleHeight,
        paddleWidth: paddleWidth,
    };

    console.log(initData);
    socket.send(JSON.stringify(initData));
};

socket.onmessage = (event: MessageEvent): void => {
    const data: GameData = JSON.parse(event.data);

    // Handle paddle positions
    if (data.leftPaddleData !== undefined) {
        // If leftPaddleY is undefined, initialize it
        if (leftPaddleY === undefined) {
            leftPaddleY = data.leftPaddleData;
        } else {
            if (data.clients !== undefined && data.clients < 2) {
                leftPaddleY = Math.max(0, Math.min(leftPaddleY + data.leftPaddleData, gameHeight - paddleHeight));
            } else {
                leftPaddleY = data.leftPaddleData;
            }
        }
    }

    if (data.rightPaddleData !== undefined) {
        // If rightPaddleY is undefined, initialize it
        if (rightPaddleY === undefined) {
            rightPaddleY = data.rightPaddleData;
        } else {
            if (data.clients !== undefined && data.clients < 2) {
                rightPaddleY = Math.max(0, Math.min(rightPaddleY + data.rightPaddleData, gameHeight - paddleHeight));
            } else {
                rightPaddleY = data.rightPaddleData;
            }
        }
    }

    // Handle ball position
    if (data.ball) {
        ballX = data.ball.x;
        ballY = data.ball.y;
    }

    // Handle score updates
    if (data.type === 'score') {
        leftScore = data.leftScore || 0;
        rightScore = data.rightScore || 0;
        scored = data.scored || '';
        
        // Start timer with appropriate message
        let scoringTeam = "Team";
        if (data.scored) {
            scoringTeam = data.scored === 'left' ? "Left Team" : "Right Team";
        }
        
        startTimer(3, `${scoringTeam} scored! Board will reset soon.`);
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
    let movement: MovementData | null = null;

    if (e.key === "w") {
        movement = { type: "move", direction: "up", paddle: "left" };
    } else if (e.key === "s") {
        movement = { type: "move", direction: "down", paddle: "left" };
    }

    if (movement) {
        socket.send(JSON.stringify(movement));
    }
});

// Clean up WebSocket connection when page closes
window.addEventListener('beforeunload', (event: BeforeUnloadEvent): void => {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, "Page is closing");
    }
});
