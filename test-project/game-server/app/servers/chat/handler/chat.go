package handler

import (
	"gomelo/lib"
)

type ChatHandler struct {
	app *lib.App
}

func (h *ChatHandler) Init(app *lib.App) { h.app = app }

func (h *ChatHandler) Send(ctx *lib.Context) {
	var req struct {
		Content string `json:"content"`
		RoomID  string `json:"roomId"`
	}
	ctx.Bind(&req)

	uid := ctx.Session().Get("uid")
	ctx.ResponseOK(map[string]any{
		"sent": true,
	})
}
