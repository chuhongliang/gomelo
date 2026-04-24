# Changelog

All notable changes to gomelo will be documented in this file.

## [1.3.0] - 2026-04-24

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