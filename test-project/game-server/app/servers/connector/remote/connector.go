package remote

import (
	"context"
	"gomelo/lib"
)

type ConnectorRemote struct {
	app *lib.App
}

func (r *ConnectorRemote) Init(app *lib.App) { r.app = app }

func (r *ConnectorRemote) AddUser(ctx context.Context, args struct {
	UserID string `json:"userId"` 
}) (any, error) {
	return map[string]any{"code": 0, "user": args.UserID}, nil
}
