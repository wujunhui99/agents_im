package model

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// 消息域共享常量、枚举映射、错误判定与领域辅助函数，移植自 internal/repository。
// msg-rpc 数据层自包含，不依赖 internal/repository。

const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"

	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeFile  = "file"

	MessageOriginHuman  = "human"
	MessageOriginAI     = "ai"
	MessageOriginSystem = "system"

	OrderAsc  = "asc"
	OrderDesc = "desc"
)

const (
	ConversationTypeSingle int64 = 1
	ConversationTypeGroup  int64 = 2

	ContentTypeTextValue  int64 = 1
	ContentTypeImageValue int64 = 2
	ContentTypeFileValue  int64 = 3

	MessageOriginHumanValue  int64 = 1
	MessageOriginAIValue     int64 = 2
	MessageOriginSystemValue int64 = 3
)

// outbox 事件枚举，移植自 internal/repository/message_outbox_repository.go。
const (
	OutboxEventTypeMessageCreated int64 = 1
	OutboxAggregateTypeMessage    int64 = 1

	OutboxStatusPending int64 = 1
)

func ConversationTypeValue(chatType string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(chatType)) {
	case ChatTypeSingle:
		return ConversationTypeSingle, nil
	case ChatTypeGroup:
		return ConversationTypeGroup, nil
	default:
		return 0, apperror.InvalidArgument("chat_type must be single or group")
	}
}

func ConversationTypeString(value int64) string {
	switch value {
	case ConversationTypeSingle:
		return ChatTypeSingle
	case ConversationTypeGroup:
		return ChatTypeGroup
	default:
		return strconv.FormatInt(value, 10)
	}
}

func ContentTypeValue(contentType string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case ContentTypeText:
		return ContentTypeTextValue, nil
	case ContentTypeImage:
		return ContentTypeImageValue, nil
	case ContentTypeFile:
		return ContentTypeFileValue, nil
	default:
		return 0, apperror.InvalidArgument("content_type must be text, image, or file")
	}
}

func ContentTypeString(value int64) string {
	switch value {
	case ContentTypeTextValue:
		return ContentTypeText
	case ContentTypeImageValue:
		return ContentTypeImage
	case ContentTypeFileValue:
		return ContentTypeFile
	default:
		return strconv.FormatInt(value, 10)
	}
}

func MessageOriginValue(origin string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(origin)) {
	case "", MessageOriginHuman:
		return MessageOriginHumanValue, nil
	case MessageOriginAI:
		return MessageOriginAIValue, nil
	case MessageOriginSystem:
		return MessageOriginSystemValue, nil
	default:
		return 0, apperror.InvalidArgument("message_origin must be human, ai, or system")
	}
}

func MessageOriginString(value int64) string {
	switch value {
	case MessageOriginHumanValue:
		return MessageOriginHuman
	case MessageOriginAIValue:
		return MessageOriginAI
	case MessageOriginSystemValue:
		return MessageOriginSystem
	default:
		return fmt.Sprintf("unknown:%d", value)
	}
}

const (
	pgUniqueViolationCode = "23505"
	pgCheckViolationCode  = "23514"
)

func isPostgresCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func IsPostgresUniqueViolation(err error) bool {
	return isPostgresCode(err, pgUniqueViolationCode)
}

func IsPostgresCheckViolation(err error) bool {
	return isPostgresCode(err, pgCheckViolationCode)
}

// SingleConversationID / GroupConversationID 复刻 internal/repository 的会话 id 约定。
func SingleConversationID(userA, userB string) string {
	lower, higher := orderedSingleUsers(userA, userB)
	return "single:" + lower + ":" + higher
}

func GroupConversationID(groupID string) string { return "group:" + groupID }

func orderedSingleUsers(userA, userB string) (string, string) {
	if userA <= userB {
		return userA, userB
	}
	return userB, userA
}

func unreadCountFromVisibleStart(maxSeq, hasReadSeq, visibleStartSeq int64) int64 {
	if hasReadSeq < visibleStartSeq {
		hasReadSeq = visibleStartSeq
	}
	if maxSeq <= hasReadSeq {
		return 0
	}
	return maxSeq - hasReadSeq
}

// VisibleUserIDs 计算会话可见成员集合（去重排序，含发送者/单聊接收者）。
func VisibleUserIDs(senderID, receiverID, chatType string, participants []string) []string {
	seen := make(map[string]struct{})
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, id := range participants {
		add(id)
	}
	add(senderID)
	if chatType == ChatTypeSingle {
		add(receiverID)
	}
	users := make([]string, 0, len(seen))
	for id := range seen {
		users = append(users, id)
	}
	sort.Strings(users)
	return users
}
