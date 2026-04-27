package lib

import (
	"context"
	"fmt"
)

type RPCProxy struct {
	app *App
}

func (p *RPCProxy) Game() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "game"}
}

func (p *RPCProxy) Gate() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "gate"}
}

func (p *RPCProxy) Chat() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "chat"}
}

func (p *RPCProxy) Match() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "match"}
}

func (p *RPCProxy) Connector() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "connector"}
}

func (p *RPCProxy) Area() *ServiceProxy {
	return &ServiceProxy{app: p.app, serverType: "area"}
}

type ServiceProxy struct {
	app        *App
	serverType string
}

func (s *ServiceProxy) Call(method string, args, reply any) error {
	if s.app.rpcMgr == nil {
		return fmt.Errorf("rpc client manager not initialized")
	}
	client, err := s.app.rpcMgr.GetClient(s.serverType)
	if err != nil {
		return err
	}
	return client.Invoke(s.serverType, method, args, reply)
}

func (s *ServiceProxy) CallCtx(ctx context.Context, method string, args, reply any) error {
	if s.app.rpcMgr == nil {
		return fmt.Errorf("rpc client manager not initialized")
	}
	client, err := s.app.rpcMgr.GetClient(s.serverType)
	if err != nil {
		return err
	}
	return client.InvokeCtx(ctx, s.serverType, method, args, reply)
}

func (s *ServiceProxy) Notify(method string, args any) error {
	if s.app.rpcMgr == nil {
		return fmt.Errorf("rpc client manager not initialized")
	}
	client, err := s.app.rpcMgr.GetClient(s.serverType)
	if err != nil {
		return err
	}
	return client.Notify(s.serverType, method, args)
}

func (s *ServiceProxy) ToServer(serverID, method string, args, reply any) error {
	return s.ToServerCtx(context.Background(), serverID, method, args, reply)
}

func (s *ServiceProxy) ToServerCtx(ctx context.Context, serverID, method string, args, reply any) error {
	if s.app.rpcMgr == nil {
		return fmt.Errorf("rpc client manager not initialized")
	}
	client, err := s.app.rpcMgr.GetClient(s.serverType)
	if err != nil {
		return err
	}
	return client.InvokeCtx(ctx, serverID, method, args, reply)
}

func (a *App) RPC() *RPCProxy {
	return &RPCProxy{app: a}
}