package handler

import (
	"gomelo/lib"
)

type GameHandler struct {
	app *lib.App
}

func (h *GameHandler) Init(app *lib.App) { h.app = app }

func (h *GameHandler) Start(ctx *lib.Context) {
	var req struct {
		RoomID string `json:"roomId"`
	}
	ctx.Bind(&req)
	ctx.ResponseOK(map[string]any{"roomId": req.RoomID})
}
