package handler

import (
	"gomelo/lib"
)

type EntryHandler struct {
	app *lib.App
}

func (h *EntryHandler) Init(app *lib.App) { h.app = app }

func (h *EntryHandler) Entry(ctx *lib.Context) {
	var req struct {
		Name string `json:"name"`
	}
	ctx.Bind(&req)

	ctx.Session().Set("uid", "user-"+strconv.FormatUint(ctx.Session().ID(), 10))

	ctx.ResponseOK(map[string]any{
		"uid":    ctx.Session().Get("uid"),
		"server": "connector-1",
	})
}
