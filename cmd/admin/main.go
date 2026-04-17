package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gomelo/protocol"
	"net"
	"net/http"
	"sync"
	"time"
)

var httpAddr = flag.String("http", ":3005", "HTTP listen address")
var masterAddr = flag.String("master", "127.0.0.1:3005", "Master server address")

type AdminServer struct {
	masterAddr string
	servers    map[string]*ServerStat
	sessions   map[string]*SessionInfo
	mu         sync.RWMutex
	mux        *http.ServeMux
	server     *http.Server
}

type ServerStat struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	State   string `json:"state"`
	Clients int    `json:"clients"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

type SessionInfo struct {
	ID       string `json:"id"`
	UID      string `json:"uid"`
	IP       string `json:"ip"`
	ServerID string `json:"serverId"`
}

type masterClient struct {
	conn net.Conn
	mu   sync.Mutex
}

type masterMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type serverInfo struct {
	ID         string `json:"id"`
	ServerType string `json:"serverType"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Frontend   bool   `json:"frontend"`
	State      int    `json:"state"`
	Count      int    `json:"count"`
}

func main() {
	flag.Parse()

	admin := &AdminServer{
		masterAddr: *masterAddr,
		servers:    make(map[string]*ServerStat),
		sessions:   make(map[string]*SessionInfo),
	}

	admin.mux = http.NewServeMux()
	admin.mux.HandleFunc("/api/servers", admin.listServers)
	admin.mux.HandleFunc("/api/stats", admin.getStats)
	admin.mux.HandleFunc("/api/connections", admin.getConnections)
	admin.mux.HandleFunc("/", admin.index)

	admin.server = &http.Server{
		Addr:    *httpAddr,
		Handler: admin.mux,
	}

	go admin.watchMaster()

	fmt.Printf("Admin server starting on %s\n", *httpAddr)
	admin.server.ListenAndServe()
}

func (a *AdminServer) watchMaster() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		servers, err := a.queryMasterServers()
		if err != nil {
			continue
		}

		a.mu.Lock()
		a.servers = make(map[string]*ServerStat)
		for _, s := range servers {
			a.servers[s.ID] = &ServerStat{
				ID:      s.ID,
				Type:    s.ServerType,
				State:   "online",
				Clients: s.Count,
				Host:    s.Host,
				Port:    s.Port,
			}
		}
		a.mu.Unlock()
	}
}

func (a *AdminServer) queryMasterServers() ([]serverInfo, error) {
	conn, err := net.DialTimeout("tcp", a.masterAddr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := protocol.NewNotify("master.queryServers", nil)
	if err := protocol.WriteMessage(conn, req); err != nil {
		return nil, err
	}

	data, err := protocol.ReadMessage(conn)
	if err != nil {
		return nil, err
	}

	var resp protocol.Message
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	var servers []serverInfo
	if err := json.Unmarshal(resp.Body, &servers); err != nil {
		return nil, err
	}

	return servers, nil
}

func (a *AdminServer) listServers(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	list := make([]*ServerStat, 0, len(a.servers))
	for _, s := range a.servers {
		list = append(list, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"servers": list,
	})
}

func (a *AdminServer) getStats(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	totalServers := len(a.servers)
	totalClients := 0
	for _, s := range a.servers {
		totalClients += s.Clients
	}
	a.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"totalServers": totalServers,
		"totalClients": totalClients,
		"timestamp":    time.Now().Format(time.RFC3339),
	})
}

func (a *AdminServer) getConnections(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	totalClients := 0
	for _, s := range a.servers {
		totalClients += s.Clients
	}
	a.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"count": totalClients,
	})
}

func (a *AdminServer) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>gomelo Admin</title>
<style>
body{font-family:Arial,sans-serif;margin:20px;background:#f5f5f5}
h1{color:#333}
.card{background:#fff;padding:20px;margin:10px 0;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,0.1)}
.stat{display:inline-block;margin:10px 20px}
.stat-value{font-size:32px;font-weight:bold;color:#2196F3}
.stat-label{color:#666}
th,td{padding:10px;text-align:left;border-bottom:1px solid #eee}
.server-online{color:green}
.server-offline{color:red}
</style>
</head>
<body>
<h1>gomelo Admin Console</h1>
<div class="card">
<div id="stats"></div>
</div>
<div class="card">
<h2>Servers</h2>
<div id="servers"></div>
</div>
<script>
function loadData(){
fetch('/api/stats').then(r=>r.json()).then(s=>{
document.getElementById('stats').innerHTML=
'<div class="stat"><div class="stat-value">'+s.totalServers+'</div><div class="stat-label">Servers</div></div>'+
'<div class="stat"><div class="stat-value">'+s.totalClients+'</div><div class="stat-label">Connections</div></div>'
})
fetch('/api/servers').then(r=>r.json()).then(d=>{
var html='<table><tr><th>ID</th><th>Type</th><th>Host</th><th>Port</th><th>Connections</th><th>State</th></tr>'
for(var s of d.servers){
html+='<tr><td>'+s.id+'</td><td>'+s.type+'</td><td>'+s.host+'</td><td>'+s.port+'</td><td>'+s.clients+'</td><td class="'+(s.state==='online'?'server-online':'server-offline')+'">'+s.state+'</td></tr>'
}
html+='</table>'
document.getElementById('servers').innerHTML=html||'<p>No servers connected</p>'
})
}
loadData()
setInterval(loadData,5000)
</script>
</body>
</html>`)
}
