# 更新日志

本文档记录 gomelo 的所有重要变更。

## [未发布]

### 新增
- **Unity C# 客户端** - Unity 游戏完整二进制协议支持
- **Godot GDScript 客户端** - 原生 GDScript 客户端实现
- **JavaScript 客户端** - 更新支持二进制协议
- **Protobuf 类型注册** - 自动类型注册实现 protobuf 编解码
- **真正的 Protobuf 支持** - 使用 `google.golang.org/protobuf` 实现 Protocol Buffers

### 修复
- **严重** Connector 中 Session Race Condition
- **严重** RPC 连接池泄漏（returnClient 将已关闭连接归还池中）
- **严重** Forwarder 客户端缓存永不清理
- **严重** Pool.Put 立即丢弃连接而非等待
- **高** MasterClient 断线后自动重连
- **中** RateLimiter 忙等循环改为高效的 sync.Cond 信号
- **中** TokenBucket 忙等循环改为高效的 sync.Cond 信号
- **中** HealthServer 添加单项检查超时（每项3秒，总计10秒）
- **中** Pipeline 缓存失效竞态条件修复

### 变更
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
