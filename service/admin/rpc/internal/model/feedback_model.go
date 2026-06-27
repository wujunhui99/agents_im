package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	domain "github.com/wujunhui99/agents_im/pkg/model"
)

// feedback_model.go 是 feedback 表的 admin 自有数据层 custom 方法（admin 是 feedback owner，
// #463 起 CreateFeedback 在 admin-rpc）。取代 internal/repository.{PostgresFeedbackRepository,
// MemoryFeedbackRepository}（#678，脱 internal）。bigint 列 ↔ string ID 沿用 #013/#550 的 keystone
// `::text` 约定（写 `$n::bigint`、读 `feedback_id::text`），故不复用 goctl 生成的 int64 Insert/Update。

var _ FeedbackModel = (*customFeedbackModel)(nil)

type (
	// FeedbackModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFeedbackModel.
	FeedbackModel interface {
		feedbackModel
		withSession(session sqlx.Session) FeedbackModel
		// CreateFeedback 插入一条反馈（缺 feedback_id 时生成 snowflake bigint keystone）并回读存储行。
		CreateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error)
		// ListFeedback 按 status 过滤、created_at desc 分页列出反馈。
		ListFeedback(ctx context.Context, filter FeedbackListFilter) ([]domain.Feedback, error)
		// GetFeedback 按 feedback_id 取单条反馈，不存在返回 apperror.NotFound。
		GetFeedback(ctx context.Context, feedbackID string) (domain.Feedback, error)
		// UpdateFeedback 更新 status/admin_note 并回读存储行，不存在返回 apperror.NotFound。
		UpdateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error)
	}

	customFeedbackModel struct {
		*defaultFeedbackModel
		newID func() (string, error)
	}

	// FeedbackListFilter narrows ListFeedback（query spec, not a domain entity）。
	FeedbackListFilter struct {
		Status domain.FeedbackStatus
		Limit  int
		Offset int
	}

	// feedbackRow 承接 ::text 转写后的 bigint id 列（feedback_id/user_id 以 string 读出）。
	feedbackRow struct {
		FeedbackID string         `db:"feedback_id"`
		UserID     string         `db:"user_id"`
		Category   string         `db:"category"`
		Status     string         `db:"status"`
		Title      string         `db:"title"`
		Content    string         `db:"content"`
		Contact    sql.NullString `db:"contact"`
		PageURL    sql.NullString `db:"page_url"`
		UserAgent  sql.NullString `db:"user_agent"`
		ClientMeta []byte         `db:"client_meta"`
		AdminNote  sql.NullString `db:"admin_note"`
		CreatedAt  time.Time      `db:"created_at"`
		UpdatedAt  time.Time      `db:"updated_at"`
	}
)

// NewFeedbackModel returns a model for the database table.
func NewFeedbackModel(conn sqlx.SqlConn) FeedbackModel {
	return &customFeedbackModel{
		defaultFeedbackModel: newFeedbackModel(conn),
		newID:                idgen.NewString,
	}
}

func (m *customFeedbackModel) withSession(session sqlx.Session) FeedbackModel {
	return NewFeedbackModel(sqlx.NewSqlConnFromSession(session))
}

const feedbackReturningCols = `feedback_id::text as feedback_id, user_id::text as user_id, category, status, title, content,
          contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at`

func (m *customFeedbackModel) CreateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error) {
	if strings.TrimSpace(feedback.FeedbackID) == "" {
		id, err := m.newID()
		if err != nil {
			return domain.Feedback{}, err
		}
		feedback.FeedbackID = id
	}
	if feedback.Status == "" {
		feedback.Status = domain.FeedbackStatusNew
	}
	meta, err := marshalFeedbackClientMeta(feedback.ClientMeta)
	if err != nil {
		return domain.Feedback{}, err
	}
	var row feedbackRow
	err = m.conn.QueryRowCtx(ctx, &row, `
insert into `+m.table+` (
  feedback_id, user_id, category, status, title, content,
  contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
)
values ($1::bigint, $2, $3, $4, $5, $6, nullif($7, ''), nullif($8, ''), nullif($9, ''), $10::jsonb, nullif($11, ''), now(), now())
returning `+feedbackReturningCols, feedback.FeedbackID, feedback.UserID, string(feedback.Category), string(feedback.Status), feedback.Title, feedback.Content,
		feedback.Contact, feedback.PageURL, feedback.UserAgent, string(meta), feedback.AdminNote)
	if err != nil {
		return domain.Feedback{}, mapFeedbackPostgresError(err)
	}
	return row.feedback()
}

func (m *customFeedbackModel) ListFeedback(ctx context.Context, filter FeedbackListFilter) ([]domain.Feedback, error) {
	limit := normalizeFeedbackLimit(filter.Limit, 20, 100)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := ""
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		where = "where status = $" + strconv.Itoa(len(args))
	}
	args = append(args, limit, offset)
	query := `
select ` + feedbackReturningCols + `
from ` + m.table + `
` + where + `
order by created_at desc, feedback_id desc
limit $` + strconv.Itoa(len(args)-1) + ` offset $` + strconv.Itoa(len(args)) + `
`
	var rows []feedbackRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	items := make([]domain.Feedback, 0, len(rows))
	for _, row := range rows {
		item, err := row.feedback()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *customFeedbackModel) GetFeedback(ctx context.Context, feedbackID string) (domain.Feedback, error) {
	feedbackID = strings.TrimSpace(feedbackID)
	if feedbackID == "" {
		return domain.Feedback{}, apperror.InvalidArgument("feedback_id is required")
	}
	var row feedbackRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select `+feedbackReturningCols+`
from `+m.table+`
where feedback_id = $1::bigint
`, feedbackID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return domain.Feedback{}, apperror.NotFound("feedback not found")
		}
		return domain.Feedback{}, err
	}
	return row.feedback()
}

func (m *customFeedbackModel) UpdateFeedback(ctx context.Context, feedback domain.Feedback) (domain.Feedback, error) {
	var row feedbackRow
	err := m.conn.QueryRowCtx(ctx, &row, `
update `+m.table+`
set status = $2, admin_note = nullif($3, ''), updated_at = now()
where feedback_id = $1::bigint
returning `+feedbackReturningCols, feedback.FeedbackID, string(feedback.Status), feedback.AdminNote)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return domain.Feedback{}, apperror.NotFound("feedback not found")
		}
		return domain.Feedback{}, mapFeedbackPostgresError(err)
	}
	return row.feedback()
}

func (r feedbackRow) feedback() (domain.Feedback, error) {
	meta := map[string]any{}
	if len(r.ClientMeta) > 0 {
		if err := json.Unmarshal(r.ClientMeta, &meta); err != nil {
			return domain.Feedback{}, err
		}
	}
	category, ok := domain.NormalizeFeedbackCategory(r.Category)
	if !ok {
		return domain.Feedback{}, apperror.Internal("stored feedback category is invalid")
	}
	status, ok := domain.NormalizeFeedbackStatus(r.Status)
	if !ok {
		return domain.Feedback{}, apperror.Internal("stored feedback status is invalid")
	}
	return domain.Feedback{
		FeedbackID: r.FeedbackID,
		UserID:     r.UserID,
		Category:   category,
		Status:     status,
		Title:      r.Title,
		Content:    r.Content,
		Contact:    r.Contact.String,
		PageURL:    r.PageURL.String,
		UserAgent:  r.UserAgent.String,
		ClientMeta: meta,
		AdminNote:  r.AdminNote.String,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}, nil
}

func marshalFeedbackClientMeta(meta map[string]any) ([]byte, error) {
	if meta == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(meta)
}

func mapFeedbackPostgresError(err error) error {
	switch {
	case IsUniqueViolation(err):
		return apperror.AlreadyExists("feedback already exists")
	case IsForeignKeyViolation(err):
		return apperror.NotFound("feedback user not found")
	case IsCheckViolation(err):
		return apperror.InvalidArgument("invalid feedback")
	default:
		return err
	}
}

// normalizeFeedbackLimit 把列表 limit 收敛到 [1, max]，0/负数取 fallback。
func normalizeFeedbackLimit(value int, fallback int, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}
