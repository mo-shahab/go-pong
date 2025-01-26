const socket = new WebSocket("ws://localhost:8080/ws");

socket.onopen = () => {
  console.log("Connected to WebSocket server");
};

socket.onmessage = (event) => {
  console.log("Message from server:", event.data);
  const data = JSON.parse(event.data);

  if (data.leftPaddleData !== undefined) {
    const leftPaddle = document.getElementById("left-paddle");
    const currentTop = parseFloat(getComputedStyle(leftPaddle).top); 
    const newTop = currentTop + data.leftPaddleData;

    // Ensure the paddle stays within the game container bounds
    const gameContainerHeight = document.getElementById("game-container").clientHeight;
    const paddleHeight = leftPaddle.clientHeight;
    leftPaddle.style.top = `${Math.max(0, Math.min(newTop, gameContainerHeight - paddleHeight))}px`;
  }

  if (data.rightPaddleData !== undefined) {
    const rightPaddle = document.getElementById("right-paddle");
    const currentTop = parseFloat(getComputedStyle(rightPaddle).top); 
    const newTop = currentTop + data.rightPaddleData;

    // Ensure the paddle stays within the game container bounds
    const gameContainerHeight = document.getElementById("game-container").clientHeight;
    const paddleHeight = rightPaddle.clientHeight;
    rightPaddle.style.top = `${Math.max(0, Math.min(newTop, gameContainerHeight - paddleHeight))}px`;
  }

  if (data.ball) {
    const ball = document.getElementById("ball");
    ball.style.left = `${data.ball.x}px`;
    ball.style.top = `${data.ball.y}px`;
  }
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

