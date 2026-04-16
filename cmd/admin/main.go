package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var httpAddr = flag.String("http", ":3005", "HTTP listen address")

type AdminServer struct {
	servers  map[string]*ServerStat
	sessions map[string]*SessionInfo
	mu       sync.RWMutex
	mux      *http.ServeMux
	server   *http.Server
}

type ServerStat struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	State   string `json:"state"`
	Clients int    `json:"clients"`
}

type SessionInfo struct {
	ID       string `json:"id"`
	UID      string `json:"uid"`
	IP       string `json:"ip"`
	ServerID string `json:"serverId"`
}

func main() {
	flag.Parse()

	admin := &AdminServer{
		servers:  make(map[string]*ServerStat),
		sessions: make(map[string]*SessionInfo),
	}

	admin.mux = http.NewServeMux()
	admin.mux.HandleFunc("/api/servers", admin.listServers)
	admin.mux.HandleFunc("/api/sessions", admin.listSessions)
	admin.mux.HandleFunc("/api/stat", admin.getStat)
	admin.mux.HandleFunc("/", admin.index)

	admin.server = &http.Server{
		Addr:    *httpAddr,
		Handler: admin.mux,
	}

	admin.server.ListenAndServe()
}

func (a *AdminServer) listServers(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	list := make([]*ServerStat, 0, len(a.servers))
	for _, s := range a.servers {
		list = append(list, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (a *AdminServer) listSessions(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	list := make([]*SessionInfo, 0, len(a.sessions))
	for _, s := range a.sessions {
		list = append(list, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (a *AdminServer) getStat(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	stat := map[string]any{
		"totalServers":  len(a.servers),
		"totalSessions": len(a.sessions),
		"timestamp":     time.Now().Format(time.RFC3339),
	}
	a.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stat)
}

func (a *AdminServer) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8"><title>Pomelo Admin</title>
<style>body{font-family:Arial,sans-serif;margin:20px;background:#f5f5f5}
h1{color:#333}.card{background:#fff;padding:20px;margin:10px 0;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,0.1)}
.stat{display:inline-block;margin:10px 20px}
.stat-value{font-size:32px;font-weight:bold;color:#2196F3}
.stat-label{color:#666}th,td{padding:10px;text-align:left;border-bottom:1px solid #eee}</style>
</head>
<body><h1>Pomelo Admin Console</h1>
<div class="card"><div id="stats"></div></div>
<div class="card"><div id="servers"></div></div>
<script>
function l(){fetch('/api/stat').then(r=>r.json()).then(s=>{
document.getElementById('stats').innerHTML='<div class="stat"><div class="stat-value">'+s.totalServers+'</div><div class="stat-label">Servers</div></div><div class="stat"><div class="stat-value">'+s.totalSessions+'</div><div class="stat-label">Sessions</div></div>'
})}
l();setInterval(l,5000);
</script></body></html>`)
}
