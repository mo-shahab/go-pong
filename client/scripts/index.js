const socket = new WebSocket("ws://localhost:8080/ws");

const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");

canvas.width = window.innerWidth * 0.9; 
canvas.height = window.innerHeight * 0.9; 

const gameWidth = canvas.width;
const gameHeight = canvas.height;

const paddleWidth = gameWidth * 0.01;
const paddleHeight = gameHeight * 0.15; 

let leftPaddleY = gameHeight / 2 - paddleHeight / 2;
let rightPaddleY = gameHeight / 2 - paddleHeight / 2;

let ballX = gameWidth / 2;
let ballY = gameHeight / 2;

function drawGame() {
  ctx.clearRect(0, 0, gameWidth, gameHeight);

  // Draw left paddle
  ctx.fillStyle = "white";
  ctx.fillRect(10, leftPaddleY, paddleWidth, paddleHeight);

  // Draw right paddle
  ctx.fillRect(gameWidth - 20, rightPaddleY, paddleWidth, paddleHeight);

  // Draw ball
  ctx.beginPath();
  ctx.arc(ballX, ballY, 8, 0, Math.PI * 2);
  ctx.fill();
}

drawGame();

socket.onopen = () => {
  console.log("Connected to WebSocket server");
};

socket.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.leftPaddleData !== undefined) {
    leftPaddleY = Math.max(0, Math.min(leftPaddleY + data.leftPaddleData, gameHeight - paddleHeight));
  }

  if (data.rightPaddleData !== undefined) {
    rightPaddleY = Math.max(0, Math.min(rightPaddleY + data.rightPaddleData, gameHeight - paddleHeight));
  }

  if (data.ball) {
    ballX = data.ball.x;
    ballY = data.ball.y;
  }

  drawGame();
};

socket.onclose = () => {
  console.log("Disconnected from WebSocket server");
};

socket.onerror = (error) => {
  console.error("WebSocket error:", error);
};

document.addEventListener("keydown", (e) => {
  let movement = null;

  if (e.key === "w") {
    movement = { type: "move", direction: "up", paddle: "left" };
  } else if (e.key === "s") {
    movement = { type: "move", direction: "down", paddle: "left" };
  }

  if (movement) {
    socket.send(JSON.stringify(movement));
  }
});

