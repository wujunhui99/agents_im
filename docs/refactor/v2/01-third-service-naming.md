# third 服务命名对齐（v2）

> v1（#429）把 mail 折入新服务 **third**，但只迁 go 落点、保留了 gRPC wire 契约 `mail.v1.MailService` 不变，故 goctl 生成的 client 包仍叫 `mailservice`（与 `authservice` 同族：`XxxService` 写法 → `xxxservice`）。

## 待办：`mailservice` → `thirdclient`

把 RPC client 包重命名为 `thirdclient`，让它对齐**服务名 third** 而非旧 RPC 名 mail。

- **要点**：goctl 用 proto 的 `service` 名生成 client 包；要产出 `thirdclient` 需把 proto `service MailService` 改为裸名 `service Third`（裸名才追加 `client` → `thirdclient`）。
- **代价（为何推到 v2）**：这会改 gRPC wire 契约（`mail.v1.MailService` → `third.v1.Third`），auth 经 `mailadapter` 的 client 须同步重生成 + 协调灰度;v1 阶段不值得为命名破坏线上契约。
- **顺带**：proto 文件 `service/third/rpc/mail.proto` 可一并更名（如 `third.proto`），并修正 `mail.pb.go` 内嵌 descriptor 仍指向 `service/mail/rpc` 的 cosmetic 尾巴（见 v1 `progress/02-microservices.md` §third）。
- **前置**：third 若新增其它第三方能力（SMS/push），按「一能力一 RPC service」拆，client 包各自独立，命名一并定调。
