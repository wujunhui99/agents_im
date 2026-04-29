# Product Specs Index

产品规格文档只描述业务逻辑和用户可感知行为，不绑定具体技术实现。

## 当前规格

- [agent-chat.md](./agent-chat.md)：Agent 单聊与群聊
- [agent-lifecycle.md](./agent-lifecycle.md)：Agent 创建、销毁与持久化
- [account-social-core.md](./account-social-core.md)：账号资料、认证、好友与群聊基础能力
- [auth-service.md](./auth-service.md)：Auth Service 第一阶段产品规格
- [user-service.md](./user-service.md)：User Service 第一阶段产品规格
- [message-chain.md](./message-chain.md)：消息发送、存储、拉取与已读链路产品规格
- [message-storage.md](./message-storage.md)：消息存储的幂等、顺序、拉取与已读保证
- [gateway-message-contract.md](./gateway-message-contract.md)：Gateway 消息命令、拉取、已读与 ACK 第一阶段客户端语义
- [frontend-sync-contract.md](./frontend-sync-contract.md)：前端重连、缺失消息同步、重复拉取与已读推进契约
- [read-receipts.md](./read-receipts.md)：标记已读、未读数和已读回执客户端行为
- [frontend-backend-contract.md](./frontend-backend-contract.md)：前端 MVP 联调的 REST、WebSocket、错误 envelope 和本地验收契约

新增需求时，先在本目录创建产品规格，再进入技术设计和执行计划。

- [backend-mvp.md](./backend-mvp.md)：后端 MVP 范围、验收标准和非 MVP 边界
