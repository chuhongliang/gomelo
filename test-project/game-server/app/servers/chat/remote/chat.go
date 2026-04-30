package remote

import (
	"context"
	"gomelo/lib"
)

type ChatRemote struct {
	app *lib.App
}

func (r *ChatRemote) Init(app *lib.App) { r.app = app }

func (r *ChatRemote) SendMessage(ctx context.Context, args struct {
	RoomID  string `json:"roomId"`
	Content string `json:"content"`
}) (any, error) {
	return map[string]any{"code": 0}, nil
}
