package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/model"
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
	CreatorUserID string    `db:"creator_account_id"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type postgresGroupMemberRow struct {
	GroupID  string       `db:"group_id"`
	UserID   string       `db:"account_id"`
	Role     int16        `db:"role"`
	Status   int16        `db:"status"`
	JoinedAt time.Time    `db:"join_time"`
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

func (r *PostgresGroupsRepository) CreateGroup(ctx context.Context, group model.Group, creatorUserID string, memberUserIDs []string) (model.Group, []model.GroupMember, error) {
	var storedGroup model.Group
	var storedMembers []model.GroupMember
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		row, err := insertGroup(ctx, session, group, creatorUserID)
		if err != nil {
			return err
		}

		seen := make(map[string]struct{}, len(memberUserIDs)+1)
		addMember := func(userID string) error {
			userID = strings.TrimSpace(userID)
			if userID == "" {
				return nil
			}
			if _, ok := seen[userID]; ok {
				return nil
			}
			seen[userID] = struct{}{}
			role := groupMemberRoleDBMember
			if userID == creatorUserID {
				role = groupMemberRoleDBOwner
			}
			memberRow, err := insertGroupMemberWithRole(ctx, session, row.GroupID, userID, role)
			if err != nil {
				return err
			}
			storedMembers = append(storedMembers, memberRow.member())
			return nil
		}
		if err := addMember(creatorUserID); err != nil {
			return err
		}
		for _, userID := range memberUserIDs {
			if err := addMember(userID); err != nil {
				return err
			}
		}

		storedGroup = row.group()
		return nil
	})
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.Group{}, nil, apperror.AlreadyExists("group already exists")
		}
		if isPostgresCheckViolation(err) {
			return model.Group{}, nil, apperror.InvalidArgument("invalid group")
		}
		if isPostgresForeignKeyViolation(err) {
			return model.Group{}, nil, apperror.NotFound("creator account not found")
		}
		return model.Group{}, nil, err
	}

	return storedGroup.Clone(), cloneGroupMembers(storedMembers), nil
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

func (r *PostgresGroupsRepository) UpdateGroup(ctx context.Context, group model.Group) (model.Group, error) {
	var row postgresGroupRow
	err := r.conn.QueryRowCtx(ctx, &row, `
update groups
set name = $2,
    description = $3,
    updated_at = now()
where group_id = $1
returning group_id, name, description, creator_account_id, created_at, updated_at
`, group.GroupID, group.Name, group.Description)
	if err != nil {
		if isNotFound(err) {
			return model.Group{}, apperror.NotFound("group not found")
		}
		if isPostgresCheckViolation(err) {
			return model.Group{}, apperror.InvalidArgument("invalid group")
		}
		return model.Group{}, err
	}
	return row.group(), nil
}

func (r *PostgresGroupsRepository) ListGroupsForUser(ctx context.Context, userID string) ([]model.Group, error) {
	var rows []postgresGroupRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select g.group_id, g.name, g.description, g.creator_account_id, g.created_at, g.updated_at
from groups g
join group_members gm on gm.group_id = g.group_id
where gm.account_id = $1 and gm.status = $2
order by g.updated_at desc, g.group_id asc
`, userID, memberStateToDB(model.MemberStateActive)); err != nil {
		return nil, err
	}

	groups := make([]model.Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, row.group())
	}
	return groups, nil
}

func (r *PostgresGroupsRepository) AddMember(ctx context.Context, groupID string, userID string) (model.GroupMember, bool, error) {
	var member model.GroupMember
	alreadyMember := false
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		groupRow, err := queryGroup(ctx, session, groupID)
		if err != nil {
			return err
		}

		existing, err := queryGroupMember(ctx, session, groupID, userID, true)
		if err == nil && memberStateFromDB(existing.Status) == model.MemberStateActive {
			member = existing.member()
			alreadyMember = true
			return nil
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		role := groupMemberRoleDBMember
		if groupRow.CreatorUserID == userID {
			role = groupMemberRoleDBOwner
		}
		row, err := upsertActiveGroupMember(ctx, session, groupID, userID, role)
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
set status = $3, left_at = now(), updated_at = now()
where group_id = $1 and account_id = $2 and status = $4
returning group_id, account_id, role, status, join_time, left_at
`, groupID, userID, memberStateToDB(model.MemberStateLeft), memberStateToDB(model.MemberStateActive))
	if err != nil {
		if isNotFound(err) {
			return model.GroupMember{}, apperror.NotFound("member not found")
		}
		return model.GroupMember{}, err
	}
	return row.member(), nil
}

func (r *PostgresGroupsRepository) RemoveMember(ctx context.Context, groupID string, userID string) (model.GroupMember, error) {
	return r.LeaveGroup(ctx, groupID, userID)
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
select group_id, account_id, role, status, join_time, left_at
from group_members
where group_id = $1 and status = $2
order by account_id asc
`, groupID, memberStateToDB(model.MemberStateActive)); err != nil {
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
		generated, genErr := idgen.NewString()
		if genErr != nil {
			return postgresGroupRow{}, genErr
		}
		group.GroupID = generated
	}
	err = session.QueryRowCtx(ctx, &row, `
insert into groups (group_id, name, description, creator_account_id)
values ($1, $2, $3, $4)
returning group_id, name, description, creator_account_id, created_at, updated_at
`, group.GroupID, group.Name, group.Description, creatorUserID)
	return row, err
}

func insertGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string) (postgresGroupMemberRow, error) {
	return insertGroupMemberWithRole(ctx, session, groupID, userID, groupMemberRoleDBOwner)
}

func insertGroupMemberWithRole(ctx context.Context, session sqlx.Session, groupID string, userID string, role int16) (postgresGroupMemberRow, error) {
	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, `
insert into group_members (group_id, account_id, role, status)
values ($1, $2, $3, $4)
returning group_id, account_id, role, status, join_time, left_at
`, groupID, userID, role, memberStateToDB(model.MemberStateActive))
	return row, err
}

func queryGroup(ctx context.Context, session sqlx.Session, groupID string) (postgresGroupRow, error) {
	var row postgresGroupRow
	err := session.QueryRowCtx(ctx, &row, `
select group_id, name, description, creator_account_id, created_at, updated_at
from groups
where group_id = $1
`, groupID)
	return row, err
}

func queryGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string, forUpdate bool) (postgresGroupMemberRow, error) {
	query := `
select group_id, account_id, role, status, join_time, left_at
from group_members
where group_id = $1 and account_id = $2
`
	if forUpdate {
		query += " for update"
	}

	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, query, groupID, userID)
	return row, err
}

func upsertActiveGroupMember(ctx context.Context, session sqlx.Session, groupID string, userID string, role int16) (postgresGroupMemberRow, error) {
	var row postgresGroupMemberRow
	err := session.QueryRowCtx(ctx, &row, `
insert into group_members (group_id, account_id, role, status)
values ($1, $2, $3, $4)
on conflict (group_id, account_id) do update
set role = excluded.role,
    status = excluded.status,
    join_time = now(),
    left_at = null,
    updated_at = now()
returning group_id, account_id, role, status, join_time, left_at
`, groupID, userID, role, memberStateToDB(model.MemberStateActive))
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
		Role:     memberRoleFromDB(r.Role),
		State:    memberStateFromDB(r.Status),
		JoinedAt: r.JoinedAt.UTC(),
	}
	if r.LeftAt.Valid {
		member.LeftAt = r.LeftAt.Time.UTC()
	}
	return member
}

func cloneGroupMembers(members []model.GroupMember) []model.GroupMember {
	cloned := make([]model.GroupMember, 0, len(members))
	for _, member := range members {
		cloned = append(cloned, member.Clone())
	}
	return cloned
}
