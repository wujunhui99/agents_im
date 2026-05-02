package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresGroupsRepository struct {
	conn sqlx.SqlConn
}

type postgresGroupRow struct {
	GroupID       string    `db:"group_id"`
	Name          string    `db:"name"`
	Description   string    `db:"description"`
	CreatorUserID string    `db:"creator_user_id"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type postgresGroupMemberRow struct {
	GroupID  string       `db:"group_id"`
	UserID   string       `db:"user_id"`
	State    string       `db:"state"`
	JoinedAt time.Time    `db:"joined_at"`
	LeftAt   sql.NullTime `db:"left_at"`
}

func NewPostgresGroupsRepository(dataSource string) (*PostgresGroupsRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresGroupsRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresGroupsRepositoryFromConn(conn sqlx.SqlConn) *PostgresGroupsRepository {
	return &PostgresGroupsRepository{conn: conn}
}

func (r *PostgresGroupsRepository) CreateGroup(ctx context.Context, group model.Group, creatorUserID string) (model.Group, model.GroupMember, error) {
	var storedGroup model.Group
	var storedMember model.GroupMember
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		row, err := insertGroup(ctx, session, group, creatorUserID)
		if err != nil {
			return err
		}

		memberRow, err := insertGroupMember(ctx, session, row.GroupID, creatorUserID)
		if err != nil {
			return err
		}

		storedGroup = row.group()
		storedMember = memberRow.member()
		return nil
	})
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.Group{}, model.GroupMember{}, apperror.AlreadyExists("group already exists")
		}
		if isPostgresCheckViolation(err) {
			return model.Group{}, model.GroupMember{}, apperror.InvalidArgument("invalid group")
		}
		if isPostgresForeignKeyViolation(err) {
			return model.Group{}, model.GroupMember{}, apperror.NotFound("creator account not found")
		}
		return model.Group{}, model.GroupMember{}, err
	}

	return storedGroup.Clone(), storedMember.Clone(), nil
}

func (r *PostgresGroupsRepository) GetGroup(ctx context.Context, groupID string) (model.Group, error) {
	row, err := queryGroup(ctx, r.conn, groupID)
	if err != nil {
		if isNotFound(err) {
			return model.Group{}, apperror.NotFound("group not found")
		}
		return model.Group{}, err
	}
	return row.group(), nil
}

func (r *PostgresGroupsRepository) AddMember(ctx context.Context, groupID string, userID string) (model.GroupMember, bool, error) {
	var member model.GroupMember
	alreadyMember := false
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := queryGroup(ctx, session, groupID); err != nil {
			return err
		}

		existing, err := queryGroupMember(ctx, session, groupID, userID, true)
		if err == nil && existing.State == model.MemberStateActive {
			member = existing.member()
			alreadyMember = true
			return nil
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		row, err := upsertActiveGroupMember(ctx, session, groupID, userID)
		if err != nil {
			return err
		}
		member = row.member()
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return model.GroupMember{}, false, apperror.NotFound("group not found")
		}
		if isPostgresForeignKeyViolation(err) {
			return model.GroupMember{}, false, apperror.NotFound("account not found")
		}
		if isPostgresCheckViolation(err) {
			return model.GroupMember{}, false, apperror.InvalidArgument("invalid group member")
		}
		return model.GroupMember{}, false, err
	}
	return member.Clone(), alreadyMember, nil
}

func (r *PostgresGroupsRepository) LeaveGroup(ctx context.Context, groupID string, userID string) (model.GroupMember, error) {
	var row postgresGroupMemberRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update group_members
set state = $3, left_at = now()
where group_id = $1 and user_id = $2 and state = $4
returning group_id, user_id, state, joined_at, left_at
`, groupID, userID, model.MemberStateLeft, model.MemberStateActive)
	if err != nil {
		if isNotFound(err) {
			return model.GroupMember{}, apperror.NotFound("member not found")
		}
		return model.GroupMember{}, err
	}
	return row.member(), nil
}

func (r *PostgresGroupsRepository) ListActiveMembers(ctx context.Context, groupID string) ([]model.GroupMember, error) {
	if _, err := queryGroup(ctx, r.conn, groupID); err != nil {
		if isNotFound(err) {
			return nil, apperror.NotFound("group not found")
		}
		return nil, err
	}

	var rows []postgresGroupMemberRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select group_id, user_id, state, joined_at, left_at
from group_members
where group_id = $1 and state = $2
order by user_id asc
`, groupID, model.MemberStateActive); err != nil {
		return nil, err
	}

	members := make([]model.GroupMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, row.member())
	}
	return members, nil
}

func insertGroup(ctx context.Context, session sqlx.Session, group model.Group, creatorUserID string) (postgresGroupRow, error) {
	var row postgresGroupRow
	var err error
	if strings.TrimSpace(group.GroupID) == "" {
		err = session.QueryRowCtx(ctx, &row, `
insert into groups (name, description, creator_user_id)
values ($1, $2, $3)
returning group_id, name, description, creator_user_id, created_at, updated_at
`, group.Name, group.Description, creatorUserID)
	} else {
		err = session.QueryRowCtx(ctx, &row, `
insert into groups (group_id, name, description, creator_user_id)
values ($1, $2, $3, $4)
returning group_id, name, description, creator_user_id, created_at, updated_at
`, group.GroupID, group.Name, group.Description, creatorUserID)
	}
	return row, err
}

func insertGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string) (postgresGroupMemberRow, error) {
	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, `
insert into group_members (group_id, user_id, state)
values ($1, $2, $3)
returning group_id, user_id, state, joined_at, left_at
`, groupID, userID, model.MemberStateActive)
	return row, err
}

func queryGroup(ctx context.Context, session sqlx.Session, groupID string) (postgresGroupRow, error) {
	var row postgresGroupRow
	err := session.QueryRowCtx(ctx, &row, `
select group_id, name, description, creator_user_id, created_at, updated_at
from groups
where group_id = $1
`, groupID)
	return row, err
}

func queryGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string, forUpdate bool) (postgresGroupMemberRow, error) {
	query := `
select group_id, user_id, state, joined_at, left_at
from group_members
where group_id = $1 and user_id = $2
`
	if forUpdate {
		query += " for update"
	}

	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, query, groupID, userID)
	return row, err
}

func upsertActiveGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string) (postgresGroupMemberRow, error) {
	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, `
insert into group_members (group_id, user_id, state)
values ($1, $2, $3)
on conflict (group_id, user_id) do update
set state = excluded.state,
    joined_at = now(),
    left_at = null
returning group_id, user_id, state, joined_at, left_at
`, groupID, userID, model.MemberStateActive)
	return row, err
}

func (r postgresGroupRow) group() model.Group {
	return model.Group{
		GroupID:       r.GroupID,
		Name:          r.Name,
		Description:   r.Description,
		CreatorUserID: r.CreatorUserID,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

func (r postgresGroupMemberRow) member() model.GroupMember {
	member := model.GroupMember{
		GroupID:  r.GroupID,
		UserID:   r.UserID,
		State:    r.State,
		JoinedAt: r.JoinedAt,
	}
	if r.LeftAt.Valid {
		member.LeftAt = r.LeftAt.Time
	}
	return member
}
