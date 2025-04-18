"use strict";
// Connect to the WebSocket server
const socket = new WebSocket("ws://localhost:8080/ws");
// Get the canvas element and its 2D context
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");
// Set canvas dimensions
canvas.width = window.innerWidth * 0.9;
canvas.height = window.innerHeight * 0.9;
// Game dimensions and settings
const gameWidth = canvas.width;
const gameHeight = canvas.height;
const paddleWidth = gameWidth * 0.01;
const paddleHeight = gameHeight * 0.15;
// Game state variables
let leftPaddleY;
let rightPaddleY;
let ballX = gameWidth / 2;
let ballY = gameHeight / 2;
let leftScore = 0;
let rightScore = 0;
// Draw the game elements
function drawGame() {
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
    // Draw scores
    ctx.font = '36px Arial';
    ctx.textAlign = 'center';
    ctx.fillText(leftScore.toString(), gameWidth / 4, 50);
    ctx.fillText(rightScore.toString(), 3 * gameWidth / 4, 50);
}
// Initial draw
drawGame();
// WebSocket event handlers
socket.onopen = () => {
    console.log("Connected to WebSocket server");
    const initData = {
        type: "init",
        width: gameWidth,
        height: gameHeight,
        paddleHeight: paddleHeight,
        paddleWidth: paddleWidth,
    };
    console.log(initData);
    socket.send(JSON.stringify(initData));
};
socket.onmessage = (event) => {
    const data = JSON.parse(event.data);
    // Handle paddle positions
    if (data.leftPaddleData !== undefined) {
        // If leftPaddleY is undefined, initialize it
        if (leftPaddleY === undefined) {
            leftPaddleY = data.leftPaddleData;
        }
        else {
            if (data.clients !== undefined && data.clients < 2) {
                leftPaddleY = Math.max(0, Math.min(leftPaddleY + data.leftPaddleData, gameHeight - paddleHeight));
            }
            else {
                leftPaddleY = data.leftPaddleData;
            }
        }
    }
    if (data.rightPaddleData !== undefined) {
        // If rightPaddleY is undefined, initialize it
        if (rightPaddleY === undefined) {
            rightPaddleY = data.rightPaddleData;
        }
        else {
            if (data.clients !== undefined && data.clients < 2) {
                rightPaddleY = Math.max(0, Math.min(rightPaddleY + data.rightPaddleData, gameHeight - paddleHeight));
            }
            else {
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
    }
    // Redraw the game with updated positions
    drawGame();
};
socket.onclose = () => {
    console.log("Disconnected from WebSocket server");
};
socket.onerror = (error) => {
    console.error("WebSocket error:", error);
};
// Keyboard event handler
document.addEventListener("keydown", (e) => {
    let movement = null;
    if (e.key === "w") {
        movement = { type: "move", direction: "up", paddle: "left" };
    }
    else if (e.key === "s") {
        movement = { type: "move", direction: "down", paddle: "left" };
    }
    if (movement) {
        socket.send(JSON.stringify(movement));
    }
});
// Clean up WebSocket connection when page closes
window.addEventListener('beforeunload', (event) => {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.close(1000, "Page is closing");
    }
});
