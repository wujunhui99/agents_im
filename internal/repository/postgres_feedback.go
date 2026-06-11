package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ FeedbackRepository = (*PostgresFeedbackRepository)(nil)

type PostgresFeedbackRepository struct {
	conn  sqlx.SqlConn
	now   func() time.Time
	newID func() (string, error)
}

type postgresFeedbackRow struct {
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

func NewFeedbackRepositoryForStorage(driver string, dataSource string) (FeedbackRepository, error) {
	storageDriver, err := repositoryStorageDriver(driver)
	if err != nil {
		return nil, err
	}
	if storageDriver == appconfig.StorageDriverMemory {
		return NewMemoryFeedbackRepository(), nil
	}
	return NewPostgresFeedbackRepository(appconfig.ResolveDataSource(dataSource))
}

func NewPostgresFeedbackRepository(dataSource string) (*PostgresFeedbackRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresFeedbackRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresFeedbackRepositoryFromConn(conn sqlx.SqlConn) *PostgresFeedbackRepository {
	return &PostgresFeedbackRepository{conn: conn, now: time.Now, newID: idgen.NewString}
}

func (r *PostgresFeedbackRepository) CreateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error) {
	if strings.TrimSpace(feedback.FeedbackID) == "" {
		id, err := r.newID()
		if err != nil {
			return model.Feedback{}, err
		}
		feedback.FeedbackID = id
	}
	if feedback.Status == "" {
		feedback.Status = model.FeedbackStatusNew
	}
	meta, err := marshalFeedbackClientMeta(feedback.ClientMeta)
	if err != nil {
		return model.Feedback{}, err
	}
	var row postgresFeedbackRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into feedback (
  feedback_id, user_id, category, status, title, content,
  contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
)
values ($1::bigint, $2, $3, $4, $5, $6, nullif($7, ''), nullif($8, ''), nullif($9, ''), $10::jsonb, nullif($11, ''), now(), now())
returning feedback_id::text as feedback_id, user_id::text as user_id, category, status, title, content,
          contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
`, feedback.FeedbackID, feedback.UserID, string(feedback.Category), string(feedback.Status), feedback.Title, feedback.Content,
		feedback.Contact, feedback.PageURL, feedback.UserAgent, string(meta), feedback.AdminNote)
	if err != nil {
		return model.Feedback{}, mapFeedbackPostgresError(err)
	}
	return row.feedback()
}

func (r *PostgresFeedbackRepository) ListFeedback(ctx context.Context, filter FeedbackListFilter) ([]model.Feedback, error) {
	limit := normalizeAdminLimit(filter.Limit, 20, 100)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := ""
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		where = "where status = $" + itoa(len(args))
	}
	args = append(args, limit, offset)
	query := `
select feedback_id::text as feedback_id, user_id::text as user_id, category, status, title, content,
       contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
from feedback
` + where + `
order by created_at desc, feedback_id desc
limit $` + itoa(len(args)-1) + ` offset $` + itoa(len(args)) + `
`
	var rows []postgresFeedbackRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	items := make([]model.Feedback, 0, len(rows))
	for _, row := range rows {
		item, err := row.feedback()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *PostgresFeedbackRepository) GetFeedback(ctx context.Context, feedbackID string) (model.Feedback, error) {
	feedbackID = strings.TrimSpace(feedbackID)
	if feedbackID == "" {
		return model.Feedback{}, apperror.InvalidArgument("feedback_id is required")
	}
	var row postgresFeedbackRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select feedback_id::text as feedback_id, user_id::text as user_id, category, status, title, content,
       contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
from feedback
where feedback_id = $1::bigint
`, feedbackID)
	if err != nil {
		if isNotFound(err) {
			return model.Feedback{}, apperror.NotFound("feedback not found")
		}
		return model.Feedback{}, err
	}
	return row.feedback()
}

func (r *PostgresFeedbackRepository) UpdateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error) {
	var row postgresFeedbackRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update feedback
set status = $2, admin_note = nullif($3, ''), updated_at = now()
where feedback_id = $1::bigint
returning feedback_id::text as feedback_id, user_id::text as user_id, category, status, title, content,
          contact, page_url, user_agent, client_meta, admin_note, created_at, updated_at
`, feedback.FeedbackID, string(feedback.Status), feedback.AdminNote)
	if err != nil {
		if isNotFound(err) {
			return model.Feedback{}, apperror.NotFound("feedback not found")
		}
		return model.Feedback{}, mapFeedbackPostgresError(err)
	}
	return row.feedback()
}

func (r postgresFeedbackRow) feedback() (model.Feedback, error) {
	meta := map[string]any{}
	if len(r.ClientMeta) > 0 {
		if err := json.Unmarshal(r.ClientMeta, &meta); err != nil {
			return model.Feedback{}, err
		}
	}
	category, ok := model.NormalizeFeedbackCategory(r.Category)
	if !ok {
		return model.Feedback{}, apperror.Internal("stored feedback category is invalid")
	}
	status, ok := model.NormalizeFeedbackStatus(r.Status)
	if !ok {
		return model.Feedback{}, apperror.Internal("stored feedback status is invalid")
	}
	return model.Feedback{
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
	if isPostgresUniqueViolation(err) {
		return apperror.AlreadyExists("feedback already exists")
	}
	if isPostgresForeignKeyViolation(err) {
		return apperror.NotFound("feedback user not found")
	}
	if isPostgresCheckViolation(err) {
		return apperror.InvalidArgument("invalid feedback")
	}
	return err
}
