package orchestrator

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/privconv"
)

// PrivateChatCoalescer 是直连 agent 私聊（#686）的合流调度器：入站 user 消息落 agent 自有
// conversation store，由 store 的 idle→running 闸门决定本次是否驱动一轮回复；一轮内把该会话所有
// 未消费的 user 消息合并成一条 user 输入（"多条追击消息一起回复"），跑完再检查 pending——若在途
// 期间又来了消息则续跑一轮，否则落 idle。全程不调 msg-rpc 拉历史（历史来自 store rounds）。
//
// 与旧 ConversationHostingService.ScheduleTrigger 的分工：私聊直连走本合流器（store 提供冪等 +
// 合流 + 历史），托管/群聊仍走 ScheduleTrigger（agent_triggers 台账 + msg-rpc 历史）。
type PrivateChatCoalescer struct {
	store      privconv.Store
	runner     AgentTriggerRunner
	readMarker AgentTriggerReadMarker
	maxRounds  int
	runTimeout time.Duration
	runningTTL time.Duration
}

// PrivateChatCoalescerConfig 见 NewPrivateChatCoalescer。
type PrivateChatCoalescerConfig struct {
	Store      privconv.Store
	Runner     AgentTriggerRunner
	ReadMarker AgentTriggerReadMarker
	MaxRounds  int
	RunTimeout time.Duration
}

// PrivateUserMessage 是一条入站私聊 user 消息（consumer 从 Kafka 事件解码后传入）。
type PrivateUserMessage struct {
	Seq         int64
	ServerMsgID string
	Text        string
	CreatedAtMs int64
}

func NewPrivateChatCoalescer(config PrivateChatCoalescerConfig) (*PrivateChatCoalescer, error) {
	if config.Store == nil {
		return nil, apperror.Internal("private chat coalescer requires a conversation store")
	}
	if config.Runner == nil {
		return nil, apperror.Internal("private chat coalescer requires a trigger runner")
	}
	maxRounds := config.MaxRounds
	if maxRounds <= 0 {
		maxRounds = privconv.DefaultMaxRounds
	}
	runTimeout := config.RunTimeout
	if runTimeout <= 0 {
		runTimeout = defaultAsyncTriggerRunTimeout
	}
	return &PrivateChatCoalescer{
		store:      config.Store,
		runner:     config.Runner,
		readMarker: config.ReadMarker,
		maxRounds:  maxRounds,
		runTimeout: runTimeout,
		runningTTL: runTimeout + time.Minute,
	}, nil
}

// OnUserMessage 把一条入站私聊消息落 store，若本次抢到 running 闸门（fired）则异步驱动 run loop。
// base 提供路由/溯源元数据（AgentUserID / RequestingUserID / ConversationID 等）；每轮的当前消息
// （TriggerSeq / TriggerMessageID / PromptText）由 run loop 从 store 的 pending 现取现合并。
func (c *PrivateChatCoalescer) OnUserMessage(ctx context.Context, base AgentTrigger, message PrivateUserMessage) error {
	if c == nil || c.store == nil {
		return apperror.Internal("private chat coalescer is not configured")
	}
	if base.ConversationType != ConversationTypeSingle {
		return apperror.InvalidArgument("private chat coalescer only supports single conversations")
	}
	if message.Seq <= 0 {
		return apperror.InvalidArgument("message seq must be greater than 0")
	}
	peerID := strings.TrimSpace(base.RequestingUserID)
	agentID := strings.TrimSpace(base.AgentUserID)
	if peerID == "" || agentID == "" {
		return apperror.InvalidArgument("private chat requires agent_user_id and requesting_user_id")
	}

	fired, err := c.store.AppendUserMessage(ctx, privconv.AppendUserMessageInput{
		ConversationID: base.ConversationID,
		AgentAccountID: agentID,
		PeerAccountID:  peerID,
		Message: privconv.PendingMessage{
			Seq:         message.Seq,
			Text:        message.Text,
			ServerMsgID: message.ServerMsgID,
			CreatedAtMs: message.CreatedAtMs,
		},
		RunningTTL: c.runningTTL,
	})
	if err != nil {
		return err
	}
	if !fired {
		return nil
	}
	c.driveAsync(base)
	return nil
}

func (c *PrivateChatCoalescer) driveAsync(base AgentTrigger) {
	go func() {
		if err := c.drive(base); err != nil {
			logx.Errorf("private chat coalescer drive failed conversation_id=%q agent_account_id=%q: %v",
				base.ConversationID, base.AgentUserID, err)
		}
	}()
}

// drive 跑合流 run loop：取当前 pending 批 → 合并成一轮 → runner.Run → CommitRound（失败则
// DiscardPending，避免对同一失败批无限重试）→ 若仍有更晚 pending 则再跑一轮，否则落 idle。
func (c *PrivateChatCoalescer) drive(base AgentTrigger) error {
	for {
		snapshot, err := c.load(base.ConversationID)
		if err != nil {
			return err
		}
		batch := snapshot.Pending
		if len(batch) == 0 {
			// running 闸门保证只有一个 loop 在跑；正常情况下 fired 时 pending 非空。
			// 兜底：无待处理批次则收束一空轮释放 running（consumedUpToSeq=0 不动 pending）。
			if _, derr := c.discard(base.ConversationID, 0); derr != nil {
				return derr
			}
			return nil
		}

		last := batch[len(batch)-1]
		trigger := c.batchTrigger(base, batch)

		runCtx, cancel := context.WithTimeout(context.Background(), c.runTimeout)
		result, runErr := c.runner.Run(runCtx, trigger)
		cancel()

		c.markRead(base, last.Seq)

		if runErr != nil {
			// 失败提示已由 runner 直接回发用户；消费掉该批避免重试风暴，不落 assistant 历史。
			logx.Errorf("private chat run failed conversation_id=%q request_id=%q seq_to=%d: %v",
				base.ConversationID, trigger.RequestID, last.Seq, runErr)
			more, derr := c.discard(base.ConversationID, last.Seq)
			if derr != nil {
				return derr
			}
			if !more {
				return nil
			}
			continue
		}

		round := privconv.Round{
			User:       pendingTexts(batch),
			Assistant:  strings.TrimSpace(result.FinalText),
			SeqFrom:    batch[0].Seq,
			SeqTo:      last.Seq,
			AgentRunID: strings.TrimSpace(result.AuditRun.RunID),
		}
		if round.Assistant == "" {
			// 正常路径 runner 已保证 final_text 非空；防御性兜底：无正文不落轮，仅消费 pending。
			more, derr := c.discard(base.ConversationID, last.Seq)
			if derr != nil {
				return derr
			}
			if !more {
				return nil
			}
			continue
		}

		more, err := c.store.CommitRound(context.Background(), privconv.CommitRoundInput{
			ConversationID:  base.ConversationID,
			Round:           round,
			ConsumedUpToSeq: last.Seq,
			MaxRounds:       c.maxRounds,
			RunningTTL:      c.runningTTL,
		})
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
}

func (c *PrivateChatCoalescer) load(conversationID string) (privconv.Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.store.Load(ctx, conversationID)
}

func (c *PrivateChatCoalescer) discard(conversationID string, consumedUpToSeq int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.store.DiscardPending(ctx, privconv.DiscardPendingInput{
		ConversationID:  conversationID,
		ConsumedUpToSeq: consumedUpToSeq,
		RunningTTL:      c.runningTTL,
	})
}

func (c *PrivateChatCoalescer) markRead(base AgentTrigger, seq int64) {
	if c.readMarker == nil || seq <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.readMarker.MarkTriggerRead(ctx, AgentTriggerReadMark{
		AccountID:      base.AgentUserID,
		ConversationID: base.ConversationID,
		TriggerSeq:     seq,
	}); err != nil {
		logx.Errorf("private chat mark-read failed conversation_id=%q agent_account_id=%q seq=%d: %v",
			base.ConversationID, base.AgentUserID, seq, err)
	}
}

// batchTrigger 从 base + 当前 pending 批组装本轮触发：当前消息=批末条（TriggerSeq/TriggerMessageID），
// PromptText=整批改行合并（合流成一条 user 输入），并置 PrivateConvContext 让请求构建器读 store。
// RequestID 追加批末 seq，保证同会话多轮 audit（agent_runs 按 RequestID）不撞。
func (c *PrivateChatCoalescer) batchTrigger(base AgentTrigger, batch []privconv.PendingMessage) AgentTrigger {
	last := batch[len(batch)-1]
	trigger := base
	trigger.RequestID = base.RequestID + ":b" + strconv.FormatInt(last.Seq, 10)
	trigger.TriggerType = TriggerTypeUserPrivateMessage
	trigger.ConversationType = ConversationTypeSingle
	trigger.TriggerMessageID = last.ServerMsgID
	trigger.TriggerSeq = last.Seq
	trigger.ReplyToMessageID = last.ServerMsgID
	trigger.PromptText = privconv.MergeTexts(batch)
	trigger.PrivateConvContext = true
	trigger.TargetAgentUserIDs = []string{base.AgentUserID}
	return trigger
}

func pendingTexts(batch []privconv.PendingMessage) []string {
	texts := make([]string, 0, len(batch))
	for _, m := range batch {
		if t := strings.TrimSpace(m.Text); t != "" {
			texts = append(texts, t)
		}
	}
	return texts
}
