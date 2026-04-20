package proto

type Player struct {
	Id    string `json:"id"`
	Name  string `json:"name"`
	Level int32  `json:"level"`
	Exp   int32  `json:"exp"`
	Gold  int64  `json:"gold"`
}

type Room struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	PlayerIds  []string `json:"player_ids"`
	MaxPlayers int32    `json:"max_players"`
	State      int32    `json:"state"`
}

type Vector3 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type Position struct {
	Pos       *Vector3 `json:"pos"`
	Rot       *Vector3 `json:"rot"`
	Timestamp int64    `json:"timestamp"`
}

type EntryRequest struct {
	Name string `json:"name"`
}

type EntryResponse struct {
	Code    int32   `json:"code"`
	Message string  `json:"message"`
	Player  *Player `json:"player"`
}

type MoveRequest struct {
	Position *Vector3 `json:"position"`
	Rotation *Vector3 `json:"rotation"`
}

type MoveResponse struct {
	Code     int32     `json:"code"`
	Position *Position `json:"position"`
}

type ChatMessage struct {
	FromId    string `json:"from_id"`
	ToId      string `json:"to_id"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Channel   int32  `json:"channel"`
}
