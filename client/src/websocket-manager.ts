import { Message } from "./proto/gopong";

type MessageHandler = (message: Message) => void;

class WebSocketManager {
    private socket: WebSocket | null = null; 
    private messageHandlers: Set<MessageHandler> = new Set();
    private reconnectionAttempts: number = 0;
    private maxReconnections: number = 5;
    private reconnectionDelay: number = 1000;
    private isIntentionallyClosed: boolean = false;

    constructor(private url: string = "ws://localhost:8080/ws") {}

    private getStoredClientId(): string | null {
        return localStorage.getItem("client-id");
    }

    connect(): Promise<void> {
        return new Promise((resolve, reject) => {
            if (this.socket?.readyState === WebSocket.OPEN) {
                resolve();
                return;
            }

            this.socket = new WebSocket(this.url);
            this.socket.binaryType = "arraybuffer";

            this.socket.onopen = async() => {
                console.log("Connected to websocket server");
                this.reconnectionAttempts = 0;
                this.reconnectionDelay = 1000;
            }

            this.socket.onmessage = async (event: MessageEvent): void => {
                console.log("Received Message from the server");
                const arrayBuffer = event.data;
                const bytes = new Uint8Array(arrayBuffer);

                const message = Message.decode(bytes);

                this.messageHandlers.forEach(handler => handler(message));
            }

            this.socket.onclose = async (event): void => {
                console.log("Websocket connect disconnected");

                if(!this.isIntentionallyClosed && this.reconnectionAttempts < this.maxReconnections) {
                    setTimeout(() => {
                        this.reconnectionAttempts++;
                        this.reconnectionDelay*=2;
                        this.connect();
                    }, this.reconnectionDelay)
                }
            }

            this.socket.onerror = async (error): void => {
                console.log("Websocket error: ", error)
                reject();
            }
        });
    }

    send(data: Uint8Array): void {
        if(this.socket?.readyState === WebSocket.OPEN){
            this.socket.send(data);
        } else {
            console.log("Websocket is not connected");
        }
    }

    addMessageHandler(handler: MessageHandler): void {
        this.messageHandlers.add(handler);
    }

    removeMessageHandler(handler: MessageHandler): void {
        this.messageHandlers.delete(handler);
    }

    isConnected(): boolean {
        return this.socket?.readyState === WebSocket.OPEN;
    }

    close(): void {
        this.isIntentionallyClosed = true;
        this.socket?.close();
    }
}

export const wsManager = new WebSocketManager();

wsManager.connect().catch(console.error);
