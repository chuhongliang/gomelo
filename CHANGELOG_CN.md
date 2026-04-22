# 更新日志

本文档记录 gomelo 的所有重要变更。

## [1.2.0] - 2026-04-22

### 新增
- **gomelo routes** - CLI 命令列出所有已注册路由
- **gomelo list** - CLI 命令显示运行中的服务器（跨平台纯 Go HTTP 实现）
- **codegen --list** - 仅列出路由不生成代码
- **ClientOptions.MaxResponseSize** - 可配置的 RPC 响应大小限制
- **Cocos Creator TypeScript 客户端** - Cocos Creator 3.x 原生 TypeScript 客户端
- **Unity C# 客户端** - Unity 游戏完整二进制协议支持
- **Godot GDScript 客户端** - 原生 GDScript 客户端实现
- **JavaScript 客户端** - 更新支持二进制协议
- **Protobuf 类型注册** - 自动类型注册实现 protobuf 编解码
- **真正的 Protobuf 支持** - 使用 `google.golang.org/protobuf` 实现 Protocol Buffers

### 修复

#### 严重并发问题
- **pool.Get()** - 检查与增加 total 非原子操作导致的竞态
- **RPCClientPool.Get()** - 同上
- **pool.Close()** - 持锁调用 Wait() 导致的死锁
- **pool.Put()** - 连接泄漏（静默丢弃而非关闭）
- **RPCClientPool.Put() timer 泄漏** - 高负载下创建大量 timer 导致 GC 压力
- **poolClient.Close()** - 持锁期间调用 Wait() 的死锁风险
- **Master reconnectLoop** - connMu 连接竞态
- **lib/app.go 事件发射** - 在 mutex unlock 后发射事件导致竞态
- **lib/app.go filter setters** - Filter getter/setter 访问 settings 无锁
- **forward/forward.go Stop()** - 清理时并发迭代 map
- **forward/forward.go cleanupLoop** - 无退出信号导致 goroutine 泄漏
- **lib/router.go Pipeline 缓存** - 双重检查锁定模式的 TOCTOU 竞态
- **lib/session.go Send/SendResponse** - 持锁期间执行 I/O
- **connector/checkHeartbeats** - 持锁期间关闭连接导致竞态
- **connector/readLoop** - 缺少 context 检查导致 goroutine 泄漏
- **connector/removeSession** - 可能双重关闭 msgCh
- **rpc/server.go handleConn** - 循环中缺少 context 检查

#### 高优先级
- **master/Heartbeat** - 在验证连接状态前设置 connected 标志
- **master/handleConn** - 静默读取错误无日志
- **master/processMessages** - 畸形输入导致缓冲区无限增长
- **master/callbacks** - 回调处理时复制前的竞态

#### 中低优先级
- **App.Set()** - 移除未使用的 `attach` 参数
- **broadcast/worker** - 添加 worker 退出时待处理任务的日志
- **RateLimiter busy-loop** - 替换为高效的 sync.Cond 信号
- **TokenBucket busy-loop** - 替换为高效的 sync.Cond 信号
- **HealthServer** - 添加单项检查超时（每项3秒，总计10秒）
- **App.afterStart** - 修复事件发射时机

### 变更
- **handleStart** - 现在实际运行服务器而非空实现
- **BuildRoute** - 输出小写路由（pomelo 兼容性）
- **模块路径** - 改为 `github.com/chuhongliang/gomelo`
- **gomelo 二进制名称** - 从 `cli` 改为 `gomelo`
- **Codec** - ProtobufCodec 使用 proto.Marshal 正确序列化
- **Codec** - 类型注册允许基于路由自动反序列化

## [1.1.0] - 2024

### 新增
- 基于 Master 协调的分布式架构
- RPC 客户端连接池
- 服务注册与发现
- 多种负载均衡策略（轮询、一致性哈希、加权随机）
- 广播服务批量消息
- 服务器间消息转发
- 超时控制的优雅关闭
- 配置热更新支持
- 熔断器模式
- 限流
- 指标采集
- 健康检查端点
- Handler/Remote 代码生成

### 组件
- `lib/` - 核心：App, Session, Message, Router, Event, Metrics, Health, Shutdown
- `rpc/` - 带连接池的 RPC 客户端
- `connector/` - 网络连接器
- `master/` - Master 协调服务器
- `registry/` - 服务注册中心
- `selector/` - 负载均衡选择器
- `broadcast/` - 广播服务
- `forward/` - 消息转发
- `pool/` - 连接池
- `loader/` - Handler/Remote 代码加载器
- `codec/` - 消息编解码（JSON/Protobuf）
- `proto/` - Protocol Buffer 消息定义
- `client/` - 客户端 SDK（JS, Godot, Unity）

## [1.0.0] - 初始版本
- 基于 Node.js Pomelo 架构的初始实现
