package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "-v", "--version":
		fmt.Printf("gomelo version %s\n", version)
	case "-h", "--help":
		printUsage()
	case "init":
		handleInit(args)
	case "start":
		handleStart(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Usage: gomelo [command]

Commands:
  init <name>    Initialize a new gomelo project
  start          Start the application
  -v, --version  Show version
  -h, --help     Show this help

Examples:
  gomelo init mygame
  cd mygame && go mod tidy && go run .
`)
}

func handleInit(args []string) {
	name := "my-game"
	if len(args) > 0 {
		name = args[0]
	}

	dir := filepath.Join(".", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	files := map[string]string{
		"main.go": mainGoTemplate,

		"go.mod": goModTemplate(name),

		"config/servers.json": serversJsonTemplate,
		"config/log.json":     logConfigTemplate,

		"servers/connector/handler/entry.go":    connectorHandlerTemplate,
		"servers/connector/remote/connector.go": connectorRemoteTemplate,
		"servers/connector/filter/time.go":      filterTemplate("connector"),
		"servers/connector/cron/auto.go":        cronTemplate("connector"),

		"servers/gate/handler/gate.go": gateHandlerTemplate,
		"servers/gate/remote/gate.go":  gateRemoteTemplate,
		"servers/gate/filter/time.go":  filterTemplate("gate"),

		"servers/chat/handler/chat.go": chatHandlerTemplate,
		"servers/chat/remote/chat.go":  chatRemoteTemplate,
		"servers/chat/filter/time.go":  filterTemplate("chat"),

		"servers/game/handler/game.go": gameHandlerTemplate,
		"servers/game/remote/game.go":  gameRemoteTemplate,
		"servers/game/filter/time.go":  filterTemplate("game"),

		"components/.gitkeep": "",
		"logs/.gitkeep":       "",
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		parent := filepath.Dir(path)
		if err := os.MkdirAll(parent, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
		if content == "" {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	modPath := filepath.Join(dir, "go.mod")
	modContent, _ := os.ReadFile(modPath)
	exeDir, _ := os.Executable()
	gomeloDir := filepath.Dir(exeDir)
	absGomelo := filepath.ToSlash(gomeloDir)
	replaceLine := fmt.Sprintf("\nreplace gomelo => %s\n", absGomelo)
	modContent = append(modContent, []byte(replaceLine)...)
	os.WriteFile(modPath, modContent, 0644)

	fmt.Printf("\nProject '%s' initialized successfully!\n\n", name)
	printDirStructure()
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", name)
	fmt.Println("  go mod tidy")
	fmt.Println("  go run .")
}

func printDirStructure() {
	fmt.Println(`  my-game/`)
	fmt.Println(`  ├── main.go`)
	fmt.Println(`  ├── go.mod`)
	fmt.Println(`  ├── config/`)
	fmt.Println(`  │   ├── servers.json`)
	fmt.Println(`  │   └── log.json`)
	fmt.Println(`  ├── servers/`)
	fmt.Println(`  │   ├── connector/`)
	fmt.Println(`  │   │   ├── handler/`)
	fmt.Println(`  │   │   │   └── entry.go`)
	fmt.Println(`  │   │   ├── remote/`)
	fmt.Println(`  │   │   │   └── connector.go`)
	fmt.Println(`  │   │   ├── filter/`)
	fmt.Println(`  │   │   │   └── time.go`)
	fmt.Println(`  │   │   └── cron/`)
	fmt.Println(`  │   │       └── auto.go`)
	fmt.Println(`  │   ├── gate/`)
	fmt.Println(`  │   │   ├── handler/`)
	fmt.Println(`  │   │   ├── remote/`)
	fmt.Println(`  │   │   └── filter/`)
	fmt.Println(`  │   ├── chat/`)
	fmt.Println(`  │   │   ├── handler/`)
	fmt.Println(`  │   │   ├── remote/`)
	fmt.Println(`  │   │   └── filter/`)
	fmt.Println(`  │   └── game/`)
	fmt.Println(`  │       ├── handler/`)
	fmt.Println(`  │       ├── remote/`)
	fmt.Println(`  │       └── filter/`)
	fmt.Println(`  ├── components/`)
	fmt.Println(`  └── logs/`)
}

func handleStart(args []string) {
	fmt.Println("Starting gomelo server...")
}

var mainGoTemplate = `package main

import (
	"log"

	"gomelo"
)

func main() {
	app := gomelo.NewApp(
		gomelo.WithHost("0.0.0.0"),
		gomelo.WithPort(3010),
		gomelo.WithServerID("connector-1"),
	)

	app.Configure("connector", "connector")(func(s *gomelo.Server) {
		s.SetFrontend(true)
		s.SetPort(3010)
	})

	app.On("connector.entry", handleEntry)

	log.Println("Starting gomelo server...")
	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Server started!")
	})

	app.Wait()
}

func handleEntry(ctx *gomelo.Context) {
	var req struct {
		Name string
	}
	ctx.Bind(&req)

	ctx.Session().Set("uid", "user-"+ctx.Session().UID())
	ctx.ResponseOK(map[string]any{
		"uid": ctx.Session().Get("uid"),
	})
}
`

func goModTemplate(name string) string {
	return fmt.Sprintf("module github.com/user/%s\n\ngo 1.21\n\nrequire gomelo v0.0.0\n", name)
}

var configJsonTemplate = `{
  "server": {
    "host": "0.0.0.0",
    "port": 3010,
    "env": "development"
  },
  "rpc": {
    "host": "0.0.0.0",
    "port": 3030,
    "maxConns": 10
  },
  "registry": {
    "host": "0.0.0.0",
    "port": 3040
  },
  "log": {
    "level": "debug",
    "path": "./logs"
  }
}
`

func serverConfigTemplate(serverType string, port int, frontend bool) string {
	frontendStr := "false"
	if frontend {
		frontendStr = "true"
	}
	return fmt.Sprintf(`{
  "server": {
    "host": "0.0.0.0",
    "port": %d,
    "serverId": "%s-1",
    "serverType": "%s",
    "frontend": %s
  },
  "log": {
    "level": "info"
  }
}
`, port, serverType, serverType, frontendStr)
}

var masterConfigTemplate = `{
  "development": {
    "id": "master-server-1",
    "host": "127.0.0.1",
    "port": 3005
  },
  "production": {
    "id": "master-server-1",
    "host": "127.0.0.1",
    "port": 3005
  }
}
`

var logConfigTemplate = `{
  "level": "info",
  "path": "./logs",
  "console": true,
  "format": "json",
  "rotate": {
    "enabled": true,
    "maxSize": 10485760,
    "maxFiles": 5,
    "maxAge": 7
  },
  "categories": {
    "default": {
      "level": "info"
    },
    "rpc": {
      "level": "error"
    },
    "connector": {
      "level": "info"
    },
    "game": {
      "level": "debug"
    }
  }
}
`

var log4jsTemplate = `{
  "appenders": [
    {
      "type": "console"
    },
    {
      "type": "file",
      "filename": "./logs/con-log-${serverId}.log",
      "pattern": "connector",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "con-log"
    },
    {
      "type": "file",
      "filename": "./logs/rpc-log-${serverId}.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "rpc-log"
    },
    {
      "type": "file",
      "filename": "./logs/forward-log-${serverId}.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "forward-log"
    },
    {
      "type": "file",
      "filename": "./logs/rpc-debug-${serverId}.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "rpc-debug"
    },
    {
      "type": "file",
      "filename": "./logs/crash.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "crash-log"
    },
    {
      "type": "file",
      "filename": "./logs/admin.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "admin-log"
    },
    {
      "type": "file",
      "filename": "./logs/pomelo-${serverId}.log",
      "maxLogSize": 1048576,
      "layout": {
        "type": "basic"
      },
      "backups": 5,
      "category": "pomelo"
    }
  ],
  "levels": {
    "rpc-log": "ERROR",
    "forward-log": "ERROR"
  },
  "replaceConsole": true,
  "lineDebug": false
}
`

var serversJsonTemplate = `{
  "development": {
    "connector": [
      {"id": "connector-server-1", "host": "127.0.0.1", "port": 3150, "clientHost": "127.0.0.1", "clientPort": 3010, "frontend": true}
    ],
    "gate": [
      {"id": "gate-server-1", "host": "127.0.0.1", "port": 3151, "clientHost": "127.0.0.1", "clientPort": 3011, "frontend": true}
    ],
    "chat": [
      {"id": "chat-server-1", "host": "127.0.0.1", "port": 3152, "frontend": false}
    ],
    "game": [
      {"id": "game-server-1", "host": "127.0.0.1", "port": 3153, "frontend": false}
    ]
  },
  "production": {
    "connector": [
      {"id": "connector-server-1", "host": "127.0.0.1", "port": 3150, "clientHost": "127.0.0.1", "clientPort": 3010, "frontend": true}
    ],
    "gate": [
      {"id": "gate-server-1", "host": "127.0.0.1", "port": 3151, "clientHost": "127.0.0.1", "clientPort": 3011, "frontend": true}
    ],
    "chat": [
      {"id": "chat-server-1", "host": "127.0.0.1", "port": 3152, "frontend": false}
    ],
    "game": [
      {"id": "game-server-1", "host": "127.0.0.1", "port": 3153, "frontend": false}
    ]
  }
}
`

var connectorHandlerTemplate = `package handler

import (
	"gomelo/lib"
)

type EntryHandler struct {
	app *lib.App
}

func (h *EntryHandler) Init(app *lib.App) { h.app = app }

func (h *EntryHandler) Entry(ctx *lib.Context) {
	var req struct {
		Name string
	}
	ctx.Bind(&req)

	ctx.Session().Set("uid", "user-"+ctx.Session().UID())
	ctx.ResponseOK(map[string]any{
		"uid": ctx.Session().Get("uid"),
	})
}
`

var connectorRemoteTemplate = `package remote

import (
	"context"
	"gomelo/lib"
)

type ConnectorRemote struct {
	app *lib.App
}

func (r *ConnectorRemote) Init(app *lib.App) { r.app = app }

func (r *ConnectorRemote) AddUser(ctx context.Context, args struct {
	UserID string
}) (any, error) {
	return map[string]any{"code": 0, "user": args.UserID}, nil
}
`

var gateHandlerTemplate = `package handler

import (
	"gomelo/lib"
)

type GateHandler struct {
	app *lib.App
}

func (h *GateHandler) Init(app *lib.App) { h.app = app }

func (h *GateHandler) HandleBind(ctx *lib.Context) {
	ctx.ResponseOK(nil)
}
`

var gateRemoteTemplate = `package remote

import (
	"context"
	"gomelo/lib"
)

type GateRemote struct {
	app *lib.App
}

func (r *GateRemote) Init(app *lib.App) { r.app = app }
`

var chatHandlerTemplate = `package handler

import (
	"gomelo/lib"
)

type ChatHandler struct {
	app *lib.App
}

func (h *ChatHandler) Init(app *lib.App) { h.app = app }

func (h *ChatHandler) Send(ctx *lib.Context) {
	ctx.ResponseOK(map[string]any{"sent": true})
}
`

var chatRemoteTemplate = `package remote

import (
	"context"
	"gomelo/lib"
)

type ChatRemote struct {
	app *lib.App
}

func (r *ChatRemote) Init(app *lib.App) { r.app = app }
`

var gameHandlerTemplate = `package handler

import (
	"gomelo/lib"
)

type GameHandler struct {
	app *lib.App
}

func (h *GameHandler) Init(app *lib.App) { h.app = app }

func (h *GameHandler) Start(ctx *lib.Context) {
	ctx.ResponseOK(nil)
}
`

var gameRemoteTemplate = `package remote

import (
	"context"
	"gomelo/lib"
)

type GameRemote struct {
	app *lib.App
}

func (r *GameRemote) Init(app *lib.App) { r.app = app }
`

func cronTemplate(serverType string) string {
	title := strings.Title(serverType)
	return fmt.Sprintf(`package cron

import (
	"context"
	"gomelo/lib"
)

type %sCron struct {
	app *lib.App
}

func (c *%sCron) Init(app *lib.App) { c.app = app }

func (c *%sCron) Cleanup(ctx context.Context) error {
	return nil
}
`, title, title, title)
}

func filterTemplate(serverType string) string {
	title := strings.Title(serverType)
	return fmt.Sprintf(`package filter

import (
	"time"
	"gomelo/lib"
)

type %sFilter struct{}

func (f *%sFilter) Name() string { return "%s" }

func (f *%sFilter) Process(ctx *lib.Context) bool {
	ctx.Set("startTime", time.Now())
	return true
}

func (f *%sFilter) After(ctx *lib.Context) {
}
`, title, title, serverType, title, title)
}

var timeFilterTemplate = `package filter

import (
	"time"
	"gomelo/lib"
)

type TimeFilter struct{}

func (f *TimeFilter) Name() string { return "time" }

func (f *TimeFilter) Process(ctx *lib.Context) bool {
	ctx.Set("startTime", time.Now())
	return true
}

func (f *TimeFilter) After(ctx *lib.Context) {
}
`
