const socket = new WebSocket("ws://localhost:8080/ws");

socket.onopen = () => {
  console.log("Connected to WebSocket server");
};

socket.onmessage = (event) => {
  console.log("Message from server:", event.data);
  const data = JSON.parse(event.data);
  console.log(data);

  if (data.leftPaddle !== undefined) {
    document.getElementById("left-paddle").style.top = `${data.leftpaddle}px`;
  }
  if (data.rightPaddle !== undefined) {
    document.getElementById("right-paddle").style.top = `${data.rightPaddle}px`;
  }
  if (data.ball) {
    document.getElementById("ball").style.left = `${data.ball.x}px`;
    document.getElementById("ball").style.top = `${data.ball.y}px`;
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

