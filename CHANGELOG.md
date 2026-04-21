# Changelog

All notable changes to gomelo will be documented in this file.

## [Unreleased]

### Added
- **Unity C# Client** - Full binary protocol support for Unity games
- **Godot GDScript Client** - Native GDScript client implementation
- **JavaScript Client** - Updated with binary protocol support
- **Protobuf Type Registry** - Automatic type registration for protobuf encoding/decoding
- **True Protobuf Support** - Using `google.golang.org/protobuf` for real Protocol Buffers

### Fixed
- **Critical** Session Race Condition in connector (handleConn)
- **Critical** RPC pool connection leak (returnClient was adding closed connections back)
- **Critical** Forwarder client cache never cleaned up
- **Critical** Pool.Put was immediately discarding connections instead of waiting
- **High** MasterClient now supports auto-reconnect on disconnection
- **Medium** RateLimiter busy-loop replaced with efficient sync.Cond signaling
- **Medium** TokenBucket busy-loop replaced with efficient sync.Cond signaling
- **Medium** HealthServer now has per-check timeouts (3s per check, 10s total)
- **Medium** Pipeline cache invalidation race condition fixed

### Changed
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
