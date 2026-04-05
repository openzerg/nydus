# nydus

事件总线与聊天室服务。OpenZerg 集群的消息基础设施。

- **端口**: 15318
- **语言**: Go
- **数据库**: SQLite (`/var/lib/nydus/nydus.db`)

## 职责

Nydus 承载两个相互独立的功能，它们共用同一个进程和数据库：

### 功能 A：服务间事件总线

集群内服务之间的异步通信信道。

- **发布者**：仅 Cerebrate 可以发布事件（通过 Cerebrate IP 身份验证）
- **订阅者**：任何服务可以订阅感兴趣的事件类型，通过长连接流式接收
- **用途**：工具权限变更通知、实例状态变更、系统级广播

```
Cerebrate ──PublishEvent──► Nydus ──Subscribe──► mutalisk
                                              ► evo-chamber
                                              ► 其他服务
```

### 功能 B：人机对话聊天室

用户与 AI Agent 之间的持久化消息信道。

- 管理员通过 `CreateChatroom` 创建对话频道，添加用户成员和 agent 实例成员
- 用户发送消息 → Nydus 路由到关联的 mutalisk 会话 → mutalisk 处理后回复 → Nydus 广播给所有订阅者
- **重要**：在新设计中，消息路由方向改为 **mutalisk 主动订阅 Nydus**（见下文）

## 消息路由：新设计 vs 旧设计

### 旧设计（ZergRepos）— 反向依赖

```
用户消息 → Nydus.SendMessage
              → nydus 内部直接调用 mutalisk HTTP API
                → mutalisk.GetOrCreateSessionByExternalId
                → mutalisk.AddMessageToSession
```

问题：Nydus（基础设施层）直接依赖 Mutalisk（应用层），依赖方向错误。Nydus 需要知道 mutalisk 的地址，且 mutalisk 必须暴露专为 Nydus 设计的内部 API。

### 新设计（OpenZergNeo）— 正确的依赖方向

```
用户消息 → Nydus.SendMessage（保存消息，广播给订阅者）
              ↑
mutalisk 主动 SubscribeMessages(chatroom_id) 接收推送
              → 触发 mutalisk 内部会话处理
              → mutalisk 回复时调用 Nydus.SendMessage
```

Mutalisk 依赖 Nydus（应用层依赖基础设施层），方向正确。Nydus 不再需要知道任何 mutalisk 的地址。

## Cerebrate IP 验证

`PublishEvent` 接口的发布者身份通过 IP 验证。在新设计中，Cerebrate IP 不再硬编码在配置文件中，而是从订阅的 ClusterState 中动态获取：

```go
// 服务启动时订阅 ClusterState
bootstrap.RegisterWithCerebrate(ctx, opts, func(state *cerebrate.ClusterState) {
    if state.Cerebrate != nil {
        nydus.SetCerebrateIP(extractIP(state.Cerebrate.URL))
    }
})
```

## API（ConnectRPC）

```protobuf
service NydusService {
    // 事件总线
    rpc PublishEvent(PublishEventRequest) returns (PublishEventResponse);
    rpc Subscribe(SubscribeRequest) returns (stream Event);
    rpc Unsubscribe(UnsubscribeRequest) returns (Empty);
    rpc ListSubscribers(ListSubscribersRequest) returns (ListSubscribersResponse);
    rpc GetEventHistory(GetEventHistoryRequest) returns (GetEventHistoryResponse);

    // 聊天室管理
    rpc CreateChatroom(CreateChatroomRequest) returns (Chatroom);
    rpc GetChatroom(GetChatroomRequest) returns (Chatroom);
    rpc UpdateChatroom(UpdateChatroomRequest) returns (Chatroom);
    rpc DeleteChatroom(DeleteChatroomRequest) returns (Empty);
    rpc ListChatrooms(ListChatroomsRequest) returns (ListChatroomsResponse);

    // 成员管理
    rpc AddMember(AddMemberRequest) returns (Member);
    rpc RemoveMember(RemoveMemberRequest) returns (Empty);
    rpc ListMembers(ListMembersRequest) returns (ListMembersResponse);
    rpc UpdateMemberRole(UpdateMemberRoleRequest) returns (Member);

    // 消息
    rpc SendMessage(SendMessageRequest) returns (Message);
    rpc GetMessageHistory(GetMessageHistoryRequest) returns (GetMessageHistoryResponse);
    rpc SubscribeMessages(SubscribeMessagesRequest) returns (stream Message);

    // 会话关联
    rpc GetChatroomSessions(GetChatroomSessionsRequest) returns (GetChatroomSessionsResponse);
}
```

## 代码结构

```
nydus/
├── cmd/nydus/
│   └── main.go
├── internal/
│   ├── config/       # 环境变量配置
│   ├── store/        # SQLite 数据访问层
│   ├── handler/
│   │   ├── event.go     # 事件总线 handler
│   │   └── chatroom.go  # 聊天室 handler
│   └── middleware/   # HTTP 中间件（token 验证、IP 提取）
├── gen/nydus/v1/     # 生成的 proto 代码
└── proto/
    └── nydus.proto
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `NYDUS_HOST` | 监听地址 | `0.0.0.0` |
| `NYDUS_PORT` | 监听端口 | `15318` |
| `NYDUS_DB_PATH` | SQLite 路径 | `/var/lib/nydus/nydus.db` |
| `NYDUS_ADMIN_TOKEN` | 管理员 Token | — |
| `CEREBRATE_URL` | Cerebrate 地址（用于注册和动态获取 Cerebrate IP） | — |
| `CEREBRATE_ADMIN_TOKEN` | Cerebrate API Key | — |
| `NYDUS_PUBLIC_URL` | 本服务公开 URL（注册到 Cerebrate） | — |

> **与 ZergRepos 的区别：** 原版有 `NYDUS_CEREBRATE_HOST` 默认值为 `10.0.0.1`（硬编码特定拓扑 IP）。新版通过 ClusterState 动态获取，不再有此配置项。
