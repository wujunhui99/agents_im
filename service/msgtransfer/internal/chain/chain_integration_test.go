//go:build integration

package chain_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/chain"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/transfer"
)

// 全链 roundtrip：publish message.submitted → toTransfer 消费（dedup + Redis seq
// Malloc + cache）→ toPostgres 消费批量落库 → 校验 PG 行/seq/dedup/重放收敛。
// 需要真实 Redpanda + Redis + PostgreSQL（migrations 已跑），缺 env 则 skip。
func TestKafkaChainRoundtrip(t *testing.T) {
	dsn := firstEnv("DATABASE_URL", "AGENTS_IM_POSTGRES_DSN")
	brokers := firstEnv("KAFKA_BROKERS")
	redisAddr := firstEnv("REDIS_ADDR")
	if dsn == "" || brokers == "" || redisAddr == "" {
		t.Skip("DATABASE_URL, KAFKA_BROKERS and REDIS_ADDR are required for kafka chain integration tests")
	}

	kafkaCfg := config.TransferKafkaConfig{
		Enabled: true,
		Brokers: brokers,
		Redis:   config.RedisConfig{Addr: redisAddr, Password: os.Getenv("REDIS_PASSWORD")},
		Workers: 4,
	}
	pipeline, err := chain.New(chain.Options{
		Kafka:      kafkaCfg,
		DataSource: dsn,
		Dispatcher: transfer.NoopDispatcher{},
		WorkerID:   "chain-it",
	})
	if err != nil {
		t.Fatalf("build chain: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := pipeline.Start(ctx); err != nil {
		t.Fatalf("start chain: %v", err)
	}
	defer pipeline.Close()

	producer, err := messaging.NewKafkaProducer(config.KafkaBrokerList(brokers))
	if err != nil {
		t.Fatalf("new producer: %v", err)
	}
	defer producer.Close()

	uniq := time.Now().UnixNano()
	sender := fmt.Sprintf("chainit-a-%d", uniq)
	receiver := fmt.Sprintf("chainit-b-%d", uniq)
	conv := fmt.Sprintf("single:%s:%s", sender, receiver)
	clientMsgID := fmt.Sprintf("chainit-cmid-%d", uniq)
	// message_id 自 #531 起为雪花 bigint：用 uniq(纳秒) 作合法 bigint server_msg_id（旧 "chainit-srv-…" 非数字会被落库解析拒绝）。
	serverMsgID := strconv.FormatInt(uniq, 10)

	event := messaging.MessageEvent{
		EventID:        fmt.Sprintf("chainit-evt-%d", uniq),
		EventType:      messaging.EventTypeMessageSubmitted,
		ConversationID: conv,
		ServerMsgID:    serverMsgID,
		SenderID:       sender,
		ChatType:       messaging.ChatTypeSingle,
		CreatedAt:      time.Now().UnixMilli(),
		Payload: messaging.MessageEventPayload{
			ClientMsgID:    clientMsgID,
			ReceiverID:     receiver,
			ContentType:    "text",
			Content:        json.RawMessage(`{"text":"hello kafka chain"}`),
			VisibleUserIDs: []string{sender, receiver},
			PayloadHash:    "chainit-hash",
			SendTime:       time.Now().UnixMilli(),
		},
	}
	if err := producer.PublishEvent(ctx, messaging.TopicToTransfer, event); err != nil {
		t.Fatalf("publish submitted: %v", err)
	}

	conn := postgres.New(dsn)
	row := waitForMessageRow(t, conn, serverMsgID, 60*time.Second)
	if row.Seq != 1 {
		t.Fatalf("expected seq 1 for fresh conversation, got %d", row.Seq)
	}
	if row.ClientMsgID != clientMsgID || row.ConversationID != conv {
		t.Fatalf("unexpected row %+v", row)
	}

	var maxSeq int64
	if err := conn.QueryRowCtx(ctx, &maxSeq, `select max_seq from conversation_threads where conversation_id = $1`, conv); err != nil {
		t.Fatalf("thread row: %v", err)
	}
	if maxSeq != 1 {
		t.Fatalf("expected thread max_seq 1, got %d", maxSeq)
	}
	var states int64
	if err := conn.QueryRowCtx(ctx, &states, `select count(*) from user_conversation_states where conversation_id = $1`, conv); err != nil {
		t.Fatalf("states count: %v", err)
	}
	if states != 2 {
		t.Fatalf("expected 2 state rows, got %d", states)
	}

	// Redis 副作用：seq 计数器与 dedup 记录。
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr, Password: os.Getenv("REDIS_PASSWORD")})
	defer rdb.Close()
	seqValue, err := rdb.Get(ctx, "msg:seq:conv:"+conv).Int64()
	if err != nil || seqValue != 1 {
		t.Fatalf("expected redis seq 1, got %d err %v", seqValue, err)
	}
	if exists, _ := rdb.Exists(ctx, fmt.Sprintf("msg:dedup:%s:%s", sender, clientMsgID)).Result(); exists != 1 {
		t.Fatalf("expected dedup key to exist")
	}

	// 重放同一条 submitted：dedup 收敛，不得产生第二行/第二个 seq。
	if err := producer.PublishEvent(ctx, messaging.TopicToTransfer, event); err != nil {
		t.Fatalf("publish duplicate: %v", err)
	}
	time.Sleep(3 * time.Second)
	var count int64
	if err := conn.QueryRowCtx(ctx, &count, `select count(*) from messages where conversation_id = $1`, conv); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 message row after duplicate, got %d", count)
	}
	seqValue, _ = rdb.Get(ctx, "msg:seq:conv:"+conv).Int64()
	if seqValue != 1 {
		t.Fatalf("expected redis seq still 1 after duplicate, got %d", seqValue)
	}
}

type messageRow struct {
	ConversationID string `db:"conversation_id"`
	ClientMsgID    string `db:"client_msg_id"`
	Seq            int64  `db:"seq"`
}

func waitForMessageRow(t *testing.T, conn sqlx.SqlConn, serverMsgID string, timeout time.Duration) messageRow {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var row messageRow
		err := conn.QueryRow(&row, `select conversation_id, client_msg_id, seq from messages where message_id = $1`, serverMsgID)
		if err == nil {
			return row
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("message %s not persisted within %s", serverMsgID, timeout)
	return messageRow{}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
