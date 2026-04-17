package selector

import (
	"testing"

	"gomelo/server_registry"
)

func TestConsistentHashSelector(t *testing.T) {
	sel := NewConsistentHashSelector(100, func(s string) int64 {
		var h int64
		for _, c := range s {
			h = h*31 + int64(c)
		}
		return h
	})

	servers := []server_registry.ServerInfo{
		{ID: "server-1", Host: "192.168.1.1", Port: 8080},
		{ID: "server-2", Host: "192.168.1.2", Port: 8080},
		{ID: "server-3", Host: "192.168.1.3", Port: 8080},
	}

	for _, s := range servers {
		sel.AddServer(s)
	}

	selected := sel.Select("user-123")
	if selected.ID == "" {
		t.Error("select returned empty server")
	}

	sel.RemoveServer(servers[0])
}
