class GameState {
    private roomId: string; 

    constructor(){}

    setRoomId(id: string) {
        this.roomId = id;
    } 

    getRoomId(): string {
        return this.roomId;
    }
}

export const gameState = new GameState();
