const URL = "ws://localhost:8080/ws";
const TOTAL_CONNECTIONS = 2;

async function connectWebSocket(index) {
    try {
        const ws = new WebSocket(URL);
        console.log(`WebSocket ${index} connecting...`);

        ws.onopen = () => {
            console.log(`WebSocket ${index} connected`);
        };

        ws.onmessage = (event) => {
            try {
                console.log(`Message from server on ${index}:`);
                const message = JSON.parse(event.data);
                console.log(message);
                
                const randomNumber = Math.random() * 100;
                const direction = randomNumber % 2 === 0 ? "up" : "down";
                const sendMessage = { type: "move", direction: direction };
                ws.send(JSON.stringify(sendMessage));
            } catch (error) {
                console.error(`Error processing message on ${index}:`, error);
            }
        };

        ws.onerror = (error) => {
            console.error(`Error on WebSocket ${index}:`, error);
        };

        ws.onclose = () => {
            console.log(`WebSocket ${index} closed`);
        };
    } catch (error) {
        console.error(`Error on WebSocket ${index}:`, error);
    }
}

function main() {
    for (let i = 0; i < TOTAL_CONNECTIONS; i++) {
        connectWebSocket(i);
    }
}

main();

