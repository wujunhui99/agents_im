# 中间件 manifests（bootstrap 事实源）

PostgreSQL / Redis / MinIO / Redpanda 的 k8s StatefulSet+Service（OB-3 已迁入 k8s，docker compose 中间件退役）。

- **不在 `deploy/k8s/kustomization.yaml` 里**：deploy-main（Drone）不管理中间件，避免应用部署
  误碰有状态服务。变更中间件需人工 `kubectl apply` 并评估数据影响。
- 由 `scripts/bootstrap-server.sh` 在新服务器引导时 apply；凭据经 `agents-im-secrets`
  （`POSTGRES_*`、`REDIS_PASSWORD`、`OBJECT_STORAGE_ACCESS_KEY_ID/SECRET_ACCESS_KEY`）。
- 历史：这些 manifests 源自 gitops 仓库 `agents_im-gitops/manifests/`（Argo CD 旁路已停用，
  2026-06-10 服务器重装后以本目录为事实源）。
- Redpanda（#470，03-message-pipeline §9 B0）：消息链路 Kafka 事实源，单 broker
  `--smp 1 --memory 400M`。与 03 文档"3-broker"目标的偏差：单物理节点上多 broker 无真实
  容错价值且内存不允许（实测见 Issue #470；同理 Redis HA 降级为单实例 AOF）。Kafka 监听
  无认证（仅 ClusterIP、单租户集群）；**禁止 dev/dev-container 模式**——developer mode 关
  fsync，会废掉 acks=all 的持久化语义。

## PostgreSQL replica

`postgres-replica.yaml` 是生产 PostgreSQL 的只读物理从库，面向
`pg-replica.agenticim.xyz:5432` 公网访问。它不由 bootstrap 或 Drone 自动 apply，因为启用它需要
operator 现场生成证书和口令，并确认主库复制配置。

安全边界：

- 只使用 PostgreSQL streaming replication，主库更新自动同步到从库；默认异步复制，允许短暂复制延迟。
- 从库启用 `hot_standby`、`default_transaction_read_only` 和 TLS。
- 公网连接必须同时通过客户端证书校验和 `scram-sha-256` 账号密码；只允许
  `agenticim_readonly` 连接。
- 客户端证书 CN 必须等于数据库用户名 `agenticim_readonly`。
- 客户端连接材料只能放在 operator-local `secret/`、服务器 `/opt/agents-im/`、k8s Secret 或密码管理器，
  不得提交到 Git、Issue、PR、聊天或日志。

上线前置：

1. 主库创建 `agenticim_replication` 复制角色、`agenticim_readonly` 只读角色，并给只读角色授予
   `pg_read_all_data`。
2. 主库 `pg_hba.conf` 允许 `agenticim_replication` 从 k3s pod CIDR 进行 replication 连接。
3. 主库创建物理复制槽 `agents_im_replica_slot`，并设置有限 `max_slot_wal_keep_size`，避免从库长时间
   掉线导致 WAL 无限占满磁盘。
4. k8s 中存在 `postgres-replica-auth`、`postgres-replica-server-tls`、`postgres-replica-client-cert`
   三个 Secret。
5. 确认宿主机 5432 端口未被占用，再人工执行：

```bash
kubectl apply -f deploy/k8s/middleware/postgres-replica.yaml
kubectl -n agents-im rollout status statefulset/postgres-replica --timeout=180s
kubectl -n agents-im get svc postgres-replica-public
```

基本验证：

```bash
kubectl -n agents-im exec statefulset/postgres -- \
  psql -U agents_im -d agents_im -c "select application_name,state,sync_state from pg_stat_replication"
kubectl -n agents-im exec statefulset/postgres-replica -- \
  psql -U agenticim_readonly -d agents_im -c "select pg_is_in_recovery(), pg_last_wal_replay_lsn()"
```
