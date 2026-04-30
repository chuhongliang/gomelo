package remote

import (
	"context"
	"gomelo/lib"
)

type GameRemote struct {
	app *lib.App
}

func (r *GameRemote) Init(app *lib.App) { r.app = app }

func (r *GameRemote) StartGame(ctx context.Context, args struct {
	RoomID string `json:"roomId"`
}) (any, error) {
	return map[string]any{"code": 0}, nil
}
