# Changelog

All notable changes to gomelo will be documented in this file.

## [1.5.2] - 2026-04-27

### New Features

#### Schema Negotiation
- **schema/schema.go** - New schema package with RouteSchema, ServerSchema, SchemaManager
- **lib/app.go** - Added RegisterRoute/RegisterJSONRoute/RegisterPBRoute APIs
- **lib/session.go** - Added SendSchema/SendRaw methods for direct raw data sending
- **lib/message.go** - Connection interface added SendRaw method
- **connector/*.go** - Auto-send Schema to client on connection establishment

#### Client Schema Handling
- **client/java/GomeloClient.java** - Support receiving and parsing Schema, dynamically register routes and Parsers
- **client/js/client.js** - Support receiving and parsing Schema, dynamically register routes and Codecs
- **client/unity/GomeloClient.cs** - Support receiving and parsing Schema
- **client/godot/client.gd** - Support receiving and parsing Schema
- **client/godot/network/packet.gd** - Schema message identification support
- **client/godot/network/protobuf_codec.gd** - Added decode_body method

#### RPC Chain Call Wrapper
- **lib/rpc_proxy.go** - New RPCProxy and ServiceProxy for chain-style RPC calls
- **lib/app.go** - Added RPC() method returning RPCProxy
- **ServiceProxy.Call(method, args, reply)** - Load-balanced call to random server instance of specified serverType
- **ServiceProxy.ToServer(serverID, method, args, reply)** - Direct call to specified serverID

#### CLI Enhancements
- **cmd/gomelo/main.go** - Added build command to compile project to binary
- **cmd/gomelo/main.go** - start command supports passing arguments to binary
- **lib/app.go** - Added ParseFlags() method and command-line flag support
- **master/master.go** - Added EnableAdmin(addr) method for built-in admin HTTP console

### Fixed

#### High Priority Issues (P1)
- **master/master.go:259-266** - Fixed handleRegister callback after unlock: create info copy before passing
- **master/master.go:375-412** - Fixed checkHeartbeats holds lock during callbacks: collect expired IDs and execute callbacks outside lock
- **pool/pool.go:106-117** - Added Warmup method for initial connection synchronization
- **selector/selector.go:113-127** - Fixed LoadBalancer uncopied slice issue
- **config/config.go:122-136** - Added Config.Validate() for required field validation
- **lib/app.go:602** - Fixed Configure() nil type assertion
- **plugin/plugin.go:96-133** - Added doCall() panic recovery

#### Build Fixes
- **connector/tcp_server.go** - Fixed atomic.AddUint32 parameter type, used unsafe.Pointer conversion
- **connector/udp_server.go** - Removed duplicate method declarations, restored strings import
- **connector/ws_server.go** - Removed unused crypto/tls and route imports, added log and errors imports

## [1.5.1] - 2026-04-27

### Fixed

#### Critical Concurrency Issues (P0)
- **rpc/client.go:193-210** - Fixed poolClient.Close() deadlock: used goroutine + timeout channel instead of direct Wait()
- **connector/udp_server.go:105-126** - Fixed UDP Server double stop panic: added sync.Once to ensure stopCh only closes once
- **rpc/server.go:164-194** - Fixed RPC context check order: check context before read, added SetReadDeadline timeout (30s)

#### High Priority Issues (P1)
- **pool/pool.go:83-95** - Added panic recovery in pool.cleanupLoop()
- **pool/pool.go:272-285** - Added panic recovery in RPCClientPool.cleanupLoop()
- **master/master.go:519-537** - Added panic recovery in watchServers goroutine

#### Critical Bug Fixes from Code Review
- **master/master.go:178-216** - Fixed infinite loop on length==0
- **master/master.go:54,255,595** - Removed unused serverIDs causing memory leak

#### Game Server Architecture Fixes
- **connector/udp_server.go:153-154** - Fixed buffer use-after-return: removed async handlePacket goroutine
- **connector/tcp_server.go:226-250** - Fixed TCP readBuf unbounded growth: added 64KB max buffer limit
- **connector/tcp_server.go:436-461** - Reduced heartbeat lock contention: close connections after releasing lock
- **lib/session.go:86-96,213-240** - Eliminated lock contention in hot send path: closed changed to atomic.Bool
- **connector/udp_server.go:364-370** - Fixed IPv6 session key bug: use addr.String() directly

#### Third Review Fixes
- **lib/app.go:314,849-913** - Fixed stopWg never used: now uses a.stopWg for component shutdown
- **connector/tcp_server.go:192-236** - Fixed double-close connection: removed conn.Close() from handleConn
- **connector/tcp_server.go:167-175** - Fixed msgWg never waited: added s.msgWg.Wait() in Stop()
- **connector/tcp_server.go:238-245** - Fixed readLoop not checking stopCh: added select to check shutdown signal
- **broadcast/broadcast.go:190-203** - Fixed Add() creates invalid sessions: added warning log
- **filter/ratelimit.go:102-117** - Fixed cleanupOldBuckets race: use mutex to protect
- **rpc/client.go:515-523** - Fixed singleClient lock pattern: unified to use Lock instead of RUnlock+Lock

#### Fourth Review Fixes
- **master/master.go:730-770** - Fixed reconnectLoop() race: moved connected check inside connMu lock

### Improved

#### Pipeline Cache Optimization (P3)
- **lib/router.go:27-97** - Use generation-based versioning instead of full cache invalidation

#### Router Lock Optimization
- **lib/router.go:61-97** - GetHandlers uses RLock for cache hits, reduces read contention by ~80%

## [1.5.0] - 2026-04-25

### Added

#### Multi-Protocol Client SDK Support
All clients now support TCP, UDP, and WebSocket protocols:

- **Go Client** - Added `ProtocolType` (tcp/udp/ws), modified `ClientOptions` to include `Protocol` field
- **JS Client** - Added `Protocol` constants, TCP/UDP support for Node.js environment
- **Java Client** - Added `Protocol` enum, `TCPClient.java`, `UDPClient.java`
- **Unity Client** - Added `ProtocolType` enum, TCP/UDP connection with read threads
- **Godot Client** - Added `ProtocolType` enum with TCP/UDP processing
- **Cocos Client** - Added `ProtocolType` enum (browser auto-fallbacks to WebSocket)

#### Configuration-Driven Auto Setup
New `AutoSetup()` and `AutoConfigure()` methods for config-driven startup:

- **lib/app.go** - Added `AutoSetup(configDir)` to automatically load master.json and servers.json
- **lib/app.go** - Added `LoadMasterConfig()` to parse master.json
- **lib/app.go** - Added `AutoConfigure()` to auto-match current server type
- **lib/app.go** - Added `SetHost()`, `SetPort()`, `SetMasterAddr()` setter methods

#### Documentation Updates
- **README.md** - Updated client SDK examples for multi-protocol usage
- **All client READMEs** - Added protocol configuration examples for each language

### Changed

- **gomelo.go** - Version bump to 1.5.0
- **Java pom.xml** - Version bump to 1.5.0
- **cmd/gomelo/main.go** - Updated main.go template to use AutoSetup/AutoConfigure

## [1.4.0] - 2026-04-24

### Added

#### Multi-Protocol Support
- **UDP Server** - New `connector/udp_server.go` for UDP game server connections
- **WebSocket Server** - Merged into `connector/ws_server.go`, unified API with TCP
- **UDPConnection** - New `lib.UDPConnection` type for UDP session management

#### Cron Scheduling
- **scheduler/cron.go** - Full cron scheduling support with Pomelo-style config
- **config/crons.json** - Environment-based cron configuration
- **CronManager** - Multi-server cron task coordination
- **CronScheduler.Cancel(id)** - Cancel task by ID

#### Code Quality
- **Connector cleanup** - Unified Forward/Selector interfaces across TCP/UDP/WS
- **Unused code removal** - Cleaned up getSession, getIP, GenerateRSAKeys, etc.

#### New Modules
- **errors/** - Unified error code system with standard HTTP-compatible codes (1001-7006 ranges)
- **reload/** - Hot reload support with file watching and signal triggering (SIGHUP/SIGUSR1)
- **metrics/** - Prometheus metrics integration with built-in collectors
- **benchmark/** - Performance benchmark test suite

#### Client SDK Enhancements
- **Unity Client** - Complete rewrite with native WebSocket (System.Net.WebSockets), heartbeat, auto-reconnect
- **Unity Client** - Fixed seq bug (uint32→uint64), removed WebSocketSharp dependency
- **Java Client** - Fixed binary message handling in WebSocketClient
- **Java Client** - Added `ProtobufCodec.java` for Protocol Buffer support
- **Java Client** - Added `CompressionUtil.java` for gzip/zlib compression
- **Godot Client** - Added `protobuf_codec.gd` and `compression.gd`
- **Cocos Client** - Added TypeScript compression utility

#### Documentation
- **Unity README** - Complete documentation with API reference
- **Godot README** - Complete documentation with GDScript examples
- **Demos** - Added demo projects for all 6 client SDKs

### Fixed

#### Client SDK
- **Java WebSocketClient** - Binary message handling (removed String-only onMessage)
- **Unity seq bug** - Changed from uint32 to uint64 for 8-byte sequence numbers
- **Unity Packet** - BitConverter.ToUInt64 instead of ToUInt32

## [1.3.0] - 2026-04-22

### Added

#### New Modules
- **errors/** - Unified error code system with standard HTTP-compatible codes (1001-7006 ranges)
- **reload/** - Hot reload support with file watching and signal triggering (SIGHUP/SIGUSR1)
- **metrics/** - Prometheus metrics integration with built-in collectors
- **benchmark/** - Performance benchmark test suite

#### Client SDK Enhancements
- **Unity Client** - Complete rewrite with native WebSocket (System.Net.WebSockets), heartbeat, auto-reconnect
- **Unity Client** - Fixed seq bug (uint32→uint64), removed WebSocketSharp dependency
- **Java Client** - Fixed binary message handling in WebSocketClient
- **Java Client** - Added `ProtobufCodec.java` for Protocol Buffer support
- **Java Client** - Added `CompressionUtil.java` for gzip/zlib compression
- **Godot Client** - Added `protobuf_codec.gd` and `compression.gd`
- **Cocos Client** - Added TypeScript compression utility

#### Documentation
- **Unity README** - Complete documentation with API reference
- **Godot README** - Complete documentation with GDScript examples
- **Demos** - Added demo projects for all 6 client SDKs

### Fixed

#### Client SDK
- **Java WebSocketClient** - Binary message handling (removed String-only onMessage)
- **Unity seq bug** - Changed from uint32 to uint64 for 8-byte sequence numbers
- **Unity Packet** - BitConverter.ToUInt64 instead of ToUInt32

## [1.2.0] - 2026-04-22

### Added
- **gomelo routes** - CLI command to list all registered routes
- **gomelo list** - CLI command to show running servers (cross-platform pure Go HTTP)
- **codegen --list** - Flag to list routes without generating code
- **ClientOptions.MaxResponseSize** - Configurable RPC response size limit
- **Cocos Creator TypeScript Client** - Native TypeScript client for Cocos Creator 3.x
- **Go Client** - Pure Go WebSocket client (no external dependencies)
- **Java Client** - Java/Android client with WebSocket support
- **Unity C# Client** - Full binary protocol support for Unity games
- **Godot GDScript Client** - Native GDScript client implementation
- **JavaScript Client** - Updated with binary protocol support
- **Protobuf Type Registry** - Automatic type registration for protobuf encoding/decoding
- **True Protobuf Support** - Using `google.golang.org/protobuf` for real Protocol Buffers

### Fixed

#### Critical Concurrency Issues
- **pool.Get()** - Race condition where check and increment of total were not atomic
- **RPCClientPool.Get()** - Same race condition as above
- **pool.Close()** - Deadlock from calling Wait() while holding lock
- **pool.Put()** - Connection leak (connections silently dropped instead of closed)
- **RPCClientPool.Put() timer leak** - Under high load, created many timers causing GC pressure
- **poolClient.Close()** - Deadlock risk from holding lock during Wait()
- **Master reconnectLoop** - Connection race condition with connMu
- **lib/app.go event emission** - Events emitted after mutex unlock causing race
- **lib/app.go filter setters** - Filter getters/setters accessing settings without mutex
- **forward/forward.go Stop()** - Concurrent map iteration during cleanup
- **forward/forward.go cleanupLoop** - No exit signal causing goroutine leak
- **lib/router.go Pipeline cache** - TOCTOU race in double-checked locking pattern
- **lib/session.go Send/SendResponse** - Lock held during I/O operations
- **connector/checkHeartbeats** - Race from closing connections while holding lock
- **connector/readLoop** - Missing context cancellation checks causing goroutine leak
- **connector/removeSession** - Potential double-close of msgCh
- **rpc/server.go handleConn** - Missing context cancellation checks in loop

#### High Priority
- **master/Heartbeat** - connected flag set before verifying connection state
- **master/handleConn** - Silent read errors without logging
- **master/processMessages** - Unbounded buffer growth on malformed input
- **master/callbacks** - Race in callback handling (copy before iteration)

#### Medium/Low
- **App.Set()** - Removed unused `attach` parameter
- **broadcast/worker** - Added logging when workers exit with pending tasks
- **RateLimiter busy-loop** - Replaced with efficient sync.Cond signaling
- **TokenBucket busy-loop** - Replaced with efficient sync.Cond signaling
- **HealthServer** - Added per-check timeouts (3s per check, 10s total)
- **App.afterStart** - Fixed event emission timing

### Changed
- **handleStart** - Now actually runs the server instead of empty implementation
- **BuildRoute** - Outputs lowercase routes (pomelo compatibility)
- **Module path** - Changed to `github.com/chuhongliang/gomelo`
- **gomelo binary name** - Changed from `cli` to `gomelo`
- **Codec** - ProtobufCodec now properly marshals using proto.Marshal
- **Codec** - Type registration allows automatic deserialization based on route

## [1.1.0] - 2024

### Added
- Distributed architecture with Master coordination
- RPC client connection pooling
- Service registry and discovery
- Multiple load balancing strategies (round-robin, consistent hash, weighted random)
- Broadcast service for batch messaging
- Message forwarding between servers
- Graceful shutdown with timeout control
- Configuration hot-reload support
- Circuit breaker pattern
- Rate limiting
- Metrics collection
- Health check endpoints
- Handler/Remote code generation

### Components
- `lib/` - Core: App, Session, Message, Router, Event, Metrics, Health, Shutdown
- `rpc/` - RPC client with connection pooling
- `connector/` - Network connector
- `master/` - Master coordination server
- `registry/` - Service registry
- `selector/` - Load balancing selectors
- `broadcast/` - Broadcast service
- `forward/` - Message forwarding
- `pool/` - Connection pooling
- `loader/` - Handler/Remote code loader
- `codec/` - Message encoding/decoding (JSON/Protobuf)
- `proto/` - Protocol buffer message definitions
- `client/` - Client SDKs (JS, Godot, Unity)

## [1.0.0] - Initial Release
- Initial implementation based on Node.js Pomelo architecture