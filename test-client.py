import asyncio
import websockets
import json
import random
import time

URL = "ws://localhost:8080/ws"
TOTAL_CONNECTIONS = 2 

async def connect_websocket(index):
    try:
        async with websockets.connect(URL) as ws:
            print(f"WebSocket {index} connected")
            while True:
                message = await ws.recv()
                print(f"Message from server on {index}: ")
                print(json.loads(message))
                random_number = random.random() * 100
                direction = "up" if random_number % 2 == 0 else "down"
                send_message = { "type": "move", "direction": direction}
                await ws.send(json.dumps(send_message))
                time.sleep(2)
    except Exception as e:
        print(f"Error on WebSocket {index}: {e}")

async def main():
    tasks = [connect_websocket(i) for i in range(TOTAL_CONNECTIONS)]
    await asyncio.gather(*tasks)

if __name__ == "__main__":
    asyncio.run(main())

