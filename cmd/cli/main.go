package main

import (
	"fmt"
	"os"
	"os/exec"
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
	case "build":
		handleBuild(args)
	case "clean":
		handleClean(args)
	case "add":
		handleAdd(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`Usage: gomelo [command] [options]

Commands:
  init <name>    Initialize a new gomelo project
  add <type>      Add a new server type (connector/chat/gate/...)
  start           Start the application
  build           Build the application
  clean           Clean build artifacts

Examples:
  gomelo init mygame
  gomelo add chat
  gomelo start

Options:
  -h, --help      Show this help message
  -v, --version   Show version info
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

	projectFiles := map[string]string{
		"main.go":                   mainGoTemplate(name),
		"go.mod":                    goModTemplate(name),
		"config.json":               configJsonTemplate,
		"config/prod.json":          prodConfigTemplate,
		"config/dev.json":           devConfigTemplate,
		"app/handlers/connector.go": connectorHandlerTemplate,
		"app/handlers/chat.go":      chatHandlerTemplate,
		"servers.json":              serversJsonTemplate,
	}

	for filename, content := range projectFiles {
		path := filepath.Join(dir, filename)
		parent := filepath.Dir(path)
		if err := os.MkdirAll(parent, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}

	modPath := filepath.Join(dir, "go.mod")
	modContent, _ := os.ReadFile(modPath)
	modContent = append(modContent, []byte("\n\nreplace gomelo => ../gomelo\n")...)
	os.WriteFile(modPath, modContent, 0644)

	fmt.Printf("Project '%s' initialized successfully!\n\n", name)
	fmt.Println("Directory structure:")
	fmt.Println("  ├── main.go              # Entry file")
	fmt.Println("  ├── go.mod               # Go module")
	fmt.Println("  ├── config.json          # Main config")
	fmt.Println("  ├── config/")
	fmt.Println("  │   ├── prod.json        # Production config")
	fmt.Println("  │   └── dev.json         # Development config")
	fmt.Println("  ├── app/")
	fmt.Println("  │   └── handlers/        # Business handlers")
	fmt.Println("  └── servers.json         # Multi-server config")
	fmt.Println("\nTo start the project:")
	fmt.Printf("  cd %s && go mod tidy && go run .\n", name)
}

func handleAdd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: gomelo add <server-type>\n")
		os.Exit(1)
	}

	serverType := args[0]
	validTypes := []string{"connector", "gate", "chat", "auth", "game", "match"}

	isValid := false
	for _, t := range validTypes {
		if serverType == t {
			isValid = true
			break
		}
	}

	if !isValid {
		fmt.Fprintf(os.Stderr, "Invalid server type: %s\n", serverType)
		fmt.Printf("Valid types: %s\n", strings.Join(validTypes, ", "))
		os.Exit(1)
	}

	handlerFile := filepath.Join("app", "handlers", serverType+".go")
	if _, err := os.Stat(handlerFile); err == nil {
		fmt.Printf("Handler '%s' already exists\n", serverType)
		return
	}

	template := handlerTemplate(serverType)
	if err := os.WriteFile(handlerFile, []byte(template), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating handler: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added server type '%s' to handlers/%s.go\n", serverType, serverType)
}

func handleStart(args []string) {
	runGo([]string{"run", "."})
}

func handleBuild(args []string) {
	runGo([]string{"build", "-o", "bin/server", "."})
}

func handleClean(args []string) {
	os.RemoveAll("bin")
	os.RemoveAll("coverage")
	fmt.Println("Cleaned build artifacts")
}

func runGo(args []string) {
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func goModTemplate(name string) string {
	return fmt.Sprintf(`module github.com/user/%s

go 1.21

require gomelo v0.0.0
`, name)
}

func mainGoTemplate(name string) string {
	return `package main

import (
	"log"
	"strconv"

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

		s.OnConnection(func(session *gomelo.Session) {
			log.Printf("New connection: %d", session.ID())
		})
	})

	app.On("connector.entry", handleEntry)

	log.Println("Starting gomelo server...")
	app.Start(func(err error) {
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Server started successfully!")
	})

	app.Wait()
}

func handleEntry(ctx *gomelo.Context) {
	var req struct {
		Name string
	}
	ctx.Bind(&req)

	ctx.Session().Set("uid", "user-"+strconv.FormatUint(ctx.Session().ID(), 10))

	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"uid":    ctx.Session().Get("uid"),
			"server": "connector-1",
		},
	})
}
`
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

var prodConfigTemplate = `{
  "server": {
    "host": "0.0.0.0",
    "port": 3010,
    "env": "production"
  },
  "log": {
    "level": "info",
    "path": "/var/log/gomelo"
  }
}
`

var devConfigTemplate = `{
  "server": {
    "host": "0.0.0.0",
    "port": 3010,
    "env": "development"
  },
  "log": {
    "level": "debug",
    "path": "./logs"
  }
}
`

var serversJsonTemplate = `{
  "development": {
    "connector": [
      {"id": "connector-1", "host": "127.0.0.1", "port": 3010}
    ],
    "gate": [
      {"id": "gate-1", "host": "127.0.0.1", "port": 3011}
    ],
    "chat": [
      {"id": "chat-1", "host": "127.0.0.1", "port": 3020}
    ]
  },
  "production": {
    "connector": [
      {"id": "connector-1", "host": "10.0.0.1", "port": 3010},
      {"id": "connector-2", "host": "10.0.0.2", "port": 3010}
    ],
    "chat": [
      {"id": "chat-1", "host": "10.0.1.1", "port": 3020},
      {"id": "chat-2", "host": "10.0.1.2", "port": 3020}
    ]
  }
}
`

var connectorHandlerTemplate = `package handlers

import (
	"gomelo/lib"
)

func HandleEntry(ctx *lib.Context) {
	var req struct {
		Token string
	}
	ctx.Bind(&req)

	if req.Token == "" {
		ctx.Response(map[string]any{"code": 401, "msg": "invalid token"})
		return
	}

	uid := "user-" + strconv.FormatUint(ctx.Session().ID(), 10)
	ctx.Session().Set("uid", uid)

	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
		"data": map[string]any{
			"uid":      uid,
			"serverId": "connector-1",
		},
	})
}
`

var chatHandlerTemplate = `package handlers

import (
	"log"
	"gomelo"
	"gomelo/lib"
)

func HandleChatSend(ctx *lib.Context) {
	var req struct {
		Content string
		RoomID  string
	}
	ctx.Bind(&req)

	uid := ctx.Session().Get("uid")
	log.Printf("Chat from %s: %s", uid, req.Content)

	broadcast := gomelo.NewBroadcast("chat.room." + req.RoomID)
	broadcast.Broadcast("chat.message", map[string]any{
		"uid":     uid,
		"content": req.Content,
	})

	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
	})
}
`

func handlerTemplate(serverType string) string {
	return fmt.Sprintf(`package handlers

import (
	"gomelo/lib"
)

func Handle%s(ctx *lib.Context) {
	ctx.Response(map[string]any{
		"code": 0,
		"msg":  "ok",
	})
}
`, strings.Title(serverType))
}
