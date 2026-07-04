// Package privconv 是 agent 私聊 conversation store 的领域封装（#686）：背靠 agent 自有
// goctl model（agent_private_conversations），把 Kafka 消费的私聊消息直写为会话上下文的
// **唯一源**，取代每次触发同步调 msg-rpc PullMessages 拉历史。
//
// 两条核心写路径：
//   - AppendUserMessage：入站 user 消息落 pending + idle→running 合流闸门（fired 决定本次是否驱动一轮）；
//   - CommitRound：一轮回复收束——追加 round、裁剪到最近 MaxRounds 轮、清掉已消费 pending，
//     残留 pending 则续 running（morePending 驱动 run loop 再跑一轮），否则落 idle。
//
// rounds/pending 的 json 形状由本包拥有（Round / PendingMessage），DB 侧只当 jsonb 存。
package privconv

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

const (
	// SummarizeThreshold：rounds 攒到该轮数触发摘要（#688）——把前 rounds-KeepRoundsAfterSummary 轮
	// 折进 long_term_memory，保留后 KeepRoundsAfterSummary 轮保证对话连续性。
	SummarizeThreshold = 20
	// KeepRoundsAfterSummary：摘要后保留的近 N 轮（连续性窗口）。
	KeepRoundsAfterSummary = 2
	// MaxMemoryChars：long_term_memory 上限（字符数，非字节），每次摘要须精简到此以内。
	MaxMemoryChars = 4096
	// MaxUserProfileChars：user_profile 上限（字符数）。
	MaxUserProfileChars = 2048

	// DefaultMaxRounds：CommitRound 的安全裁剪上限（远高于 SummarizeThreshold）。摘要正常时 rounds
	// 折到 KeepRoundsAfterSummary 不会触顶；仅当未配置 summarizer / 摘要持续失败时兜底防无限膨胀。
	DefaultMaxRounds = 40
)

// Round 是一轮 (user 批次 → assistant 回复)。User 保留合流前的多条原文（组装 prompt 时改行合并成
// 一条 user 消息，保持 user/assistant 严格交互）。
type Round struct {
	User      []string `json:"user"`
	Assistant string   `json:"assistant"`
	// SeqFrom/SeqTo 不加 omitempty：ApplySummary 按 seq_to 过滤已摘要轮，需保证该键始终序列化。
	SeqFrom    int64  `json:"seq_from"`
	SeqTo      int64  `json:"seq_to"`
	AgentRunID string `json:"agent_run_id,omitempty"`
}

// PendingMessage 是尚未被 agent 消费的一条 user 消息。
type PendingMessage struct {
	Seq         int64  `json:"seq"`
	Text        string `json:"text"`
	ServerMsgID string `json:"server_msg_id,omitempty"`
	CreatedAtMs int64  `json:"created_at_ms,omitempty"`
}

// Snapshot 是会话当前上下文（用于组装 prompt 与驱动 run loop）。
type Snapshot struct {
	ConversationID string
	AgentAccountID string
	PeerAccountID  string
	Rounds         []Round
	Pending        []PendingMessage
	State          string
	// LongTermMemory/UserProfile：达阈值摘要后折进的长期记忆与用户画像（#688），组 prompt 时注入 system 段。
	LongTermMemory string
	UserProfile    string
}

// ApplySummaryInput 见 Store.ApplySummary。
type ApplySummaryInput struct {
	ConversationID string
	LongTermMemory string
	UserProfile    string
	// CutoffSeq：seq_to<=CutoffSeq 的轮被移除（已折进 LongTermMemory），保留后续轮。<=0 时不删轮。
	CutoffSeq int64
}

// AppendUserMessageInput 见 Store.AppendUserMessage。
type AppendUserMessageInput struct {
	ConversationID string
	AgentAccountID string
	PeerAccountID  string
	Message        PendingMessage
	RunningTTL     time.Duration
}

// CommitRoundInput 见 Store.CommitRound。
type CommitRoundInput struct {
	ConversationID  string
	Round           Round
	ConsumedUpToSeq int64
	MaxRounds       int
	RunningTTL      time.Duration
}

// Store 是私聊 conversation store 的领域接口。
type Store interface {
	AppendUserMessage(ctx context.Context, in AppendUserMessageInput) (fired bool, err error)
	CommitRound(ctx context.Context, in CommitRoundInput) (morePending bool, err error)
	DiscardPending(ctx context.Context, in DiscardPendingInput) (morePending bool, err error)
	ApplySummary(ctx context.Context, in ApplySummaryInput) error
	Load(ctx context.Context, conversationID string) (Snapshot, error)
}

// DiscardPendingInput 见 Store.DiscardPending。
type DiscardPendingInput struct {
	ConversationID  string
	ConsumedUpToSeq int64
	RunningTTL      time.Duration
}

// ModelStore 是背靠 goctl model 的 Store 实现。
type ModelStore struct {
	rows model.AgentPrivateConversationsModel
}

var _ Store = (*ModelStore)(nil)

// NewModelStore 用数据源构建 model-backed Store。
func NewModelStore(dataSource string) *ModelStore {
	return NewModelStoreFromConn(postgres.New(dataSource))
}

// NewModelStoreFromConn 用已建连接构建 model-backed Store。
func NewModelStoreFromConn(conn sqlx.SqlConn) *ModelStore {
	return &ModelStore{rows: model.NewAgentPrivateConversationsModel(conn)}
}

func (s *ModelStore) AppendUserMessage(ctx context.Context, in AppendUserMessageInput) (bool, error) {
	if err := validateID(in.ConversationID, "conversation_id"); err != nil {
		return false, err
	}
	if err := validateID(in.AgentAccountID, "agent_account_id"); err != nil {
		return false, err
	}
	if err := validateID(in.PeerAccountID, "peer_account_id"); err != nil {
		return false, err
	}
	if in.Message.Seq <= 0 {
		return false, apperror.InvalidArgument("message seq must be greater than 0")
	}
	elem, err := json.Marshal(in.Message)
	if err != nil {
		return false, apperror.Internal("marshal pending message: " + err.Error())
	}
	return s.rows.AppendPending(ctx, model.AppendPendingInput{
		ConversationID:   in.ConversationID,
		AgentAccountID:   in.AgentAccountID,
		PeerAccountID:    in.PeerAccountID,
		PendingElemJSON:  string(elem),
		Seq:              in.Message.Seq,
		RunningTTLMillis: ttlMillis(in.RunningTTL),
	})
}

func (s *ModelStore) CommitRound(ctx context.Context, in CommitRoundInput) (bool, error) {
	if err := validateID(in.ConversationID, "conversation_id"); err != nil {
		return false, err
	}
	maxRounds := in.MaxRounds
	if maxRounds <= 0 {
		maxRounds = DefaultMaxRounds
	}
	elem, err := json.Marshal(in.Round)
	if err != nil {
		return false, apperror.Internal("marshal round: " + err.Error())
	}
	morePending, err := s.rows.CommitRound(ctx, model.CommitRoundInput{
		ConversationID:   in.ConversationID,
		RoundJSON:        string(elem),
		ConsumedUpToSeq:  in.ConsumedUpToSeq,
		MaxRounds:        int64(maxRounds),
		RunningTTLMillis: ttlMillis(in.RunningTTL),
	})
	if err != nil {
		if err == model.ErrNotFound {
			return false, apperror.NotFound("agent private conversation not found")
		}
		return false, err
	}
	return morePending, nil
}

func (s *ModelStore) DiscardPending(ctx context.Context, in DiscardPendingInput) (bool, error) {
	if err := validateID(in.ConversationID, "conversation_id"); err != nil {
		return false, err
	}
	morePending, err := s.rows.DiscardPending(ctx, in.ConversationID, in.ConsumedUpToSeq, ttlMillis(in.RunningTTL))
	if err != nil {
		if err == model.ErrNotFound {
			return false, apperror.NotFound("agent private conversation not found")
		}
		return false, err
	}
	return morePending, nil
}

func (s *ModelStore) ApplySummary(ctx context.Context, in ApplySummaryInput) error {
	if err := validateID(in.ConversationID, "conversation_id"); err != nil {
		return err
	}
	memory := truncateChars(in.LongTermMemory, MaxMemoryChars)
	profile := truncateChars(in.UserProfile, MaxUserProfileChars)
	if err := s.rows.ApplySummary(ctx, in.ConversationID, memory, profile, in.CutoffSeq); err != nil {
		if err == model.ErrNotFound {
			return apperror.NotFound("agent private conversation not found")
		}
		return err
	}
	return nil
}

// truncateChars 按字符数（rune）截断，保护 long_term_memory / user_profile 的存储上限。
func truncateChars(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return string(r[:maxChars])
}

// MergeTexts 把一批 user 消息按 seq 顺序的文本用改行合并成一条 user 输入（合流后的当轮 prompt）。
// 空白项跳过；用于组装"多条追击消息一起回复"的单条 user turn。
func MergeTexts(messages []PendingMessage) string {
	parts := make([]string, 0, len(messages))
	for _, m := range messages {
		if t := strings.TrimSpace(m.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}

func (s *ModelStore) Load(ctx context.Context, conversationID string) (Snapshot, error) {
	if err := validateID(conversationID, "conversation_id"); err != nil {
		return Snapshot{}, err
	}
	row, err := s.rows.FindOneByConversationId(ctx, conversationID)
	if err != nil {
		if err == model.ErrNotFound {
			return Snapshot{}, apperror.NotFound("agent private conversation not found")
		}
		return Snapshot{}, err
	}
	rounds, err := decodeRounds(row.Rounds)
	if err != nil {
		return Snapshot{}, err
	}
	pending, err := decodePending(row.Pending)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		ConversationID: row.ConversationId,
		AgentAccountID: row.AgentAccountId,
		PeerAccountID:  row.PeerAccountId,
		Rounds:         rounds,
		Pending:        pending,
		State:          row.State,
		LongTermMemory: row.LongTermMemory,
		UserProfile:    row.UserProfile,
	}, nil
}

func decodeRounds(raw string) ([]Round, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var rounds []Round
	if err := json.Unmarshal([]byte(raw), &rounds); err != nil {
		return nil, apperror.Internal("decode rounds: " + err.Error())
	}
	return rounds, nil
}

func decodePending(raw string) ([]PendingMessage, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var pending []PendingMessage
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		return nil, apperror.Internal("decode pending: " + err.Error())
	}
	return pending, nil
}

func validateID(value, field string) error {
	if strings.TrimSpace(value) == "" {
		return apperror.InvalidArgument(field + " is required")
	}
	return nil
}

func ttlMillis(ttl time.Duration) int64 {
	if ttl <= 0 {
		return 0
	}
	return ttl.Milliseconds()
}
