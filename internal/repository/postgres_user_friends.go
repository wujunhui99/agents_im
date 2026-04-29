package repository

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type postgresUserRow struct {
	UserID      string    `db:"user_id"`
	Identifier  string    `db:"identifier"`
	DisplayName string    `db:"display_name"`
	Name        string    `db:"name"`
	Gender      string    `db:"gender"`
	Age         int32     `db:"age"`
	Region      string    `db:"region"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type postgresFriendshipRow struct {
	UserID    string    `db:"user_id"`
	FriendID  string    `db:"friend_id"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r *PostgresRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	var row postgresUserRow
	var err error
	if strings.TrimSpace(user.UserID) == "" {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into users (identifier, display_name, name, gender, age, region)
values ($1, $2, $3, $4, $5, $6)
returning user_id, identifier, display_name, name, gender, age, region, created_at, updated_at
`, user.Identifier, user.DisplayName, user.Name, user.Gender, user.Age, user.Region)
	} else {
		err = r.conn.QueryRowCtx(ctx, &row, `
insert into users (user_id, identifier, display_name, name, gender, age, region)
values ($1, $2, $3, $4, $5, $6, $7)
returning user_id, identifier, display_name, name, gender, age, region, created_at, updated_at
`, user.UserID, user.Identifier, user.DisplayName, user.Name, user.Gender, user.Age, user.Region)
	}
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.User{}, apperror.AlreadyExists("identifier already exists")
		}
		if isPostgresCheckViolation(err) {
			return model.User{}, apperror.InvalidArgument("invalid user profile")
		}
		return model.User{}, err
	}

	return row.user(), nil
}

func (r *PostgresRepository) GetByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	var row postgresUserRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select user_id, identifier, display_name, name, gender, age, region, created_at, updated_at
from users
where identifier = $1
`, identifier)
	if err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("user not found")
		}
		return model.User{}, err
	}
	return row.user(), nil
}

func (r *PostgresRepository) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	var exists bool
	err := r.conn.QueryRowCtx(ctx, &exists, `
select exists(select 1 from users where identifier = $1)
`, identifier)
	return exists, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, userID string) (model.User, error) {
	var row postgresUserRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select user_id, identifier, display_name, name, gender, age, region, created_at, updated_at
from users
where user_id = $1
`, userID)
	if err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("user not found")
		}
		return model.User{}, err
	}
	return row.user(), nil
}

func (r *PostgresRepository) UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (model.User, error) {
	setters := make([]string, 0, 5)
	args := make([]any, 0, 6)
	addSetter := func(column string, value any) {
		args = append(args, value)
		setters = append(setters, column+" = $"+itoa(len(args)))
	}

	if patch.DisplayName != nil {
		addSetter("display_name", *patch.DisplayName)
	}
	if patch.Name != nil {
		addSetter("name", *patch.Name)
	}
	if patch.Gender != nil {
		addSetter("gender", *patch.Gender)
	}
	if patch.Age != nil {
		addSetter("age", *patch.Age)
	}
	if patch.Region != nil {
		addSetter("region", *patch.Region)
	}
	if len(setters) == 0 {
		return r.GetByID(ctx, userID)
	}

	args = append(args, userID)
	query := `
update users
set ` + strings.Join(setters, ", ") + `, updated_at = now()
where user_id = $` + itoa(len(args)) + `
returning user_id, identifier, display_name, name, gender, age, region, created_at, updated_at
`
	var row postgresUserRow
	if err := r.conn.QueryRowCtx(ctx, &row, query, args...); err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("user not found")
		}
		if isPostgresCheckViolation(err) {
			return model.User{}, apperror.InvalidArgument("invalid user profile")
		}
		return model.User{}, err
	}
	return row.user(), nil
}

func (r *PostgresRepository) AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	created := true
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		existing, err := queryFriendship(ctx, session, userID, friendID, true)
		if err == nil && existing.Status == model.FriendshipStatusActive {
			friendship = existing
			created = false
			return nil
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		row, err := upsertActiveFriendship(ctx, session, userID, friendID)
		if err != nil {
			return err
		}
		if _, err := upsertActiveFriendship(ctx, session, friendID, userID); err != nil {
			return err
		}
		friendship = row
		return nil
	})
	if err != nil {
		if isPostgresCheckViolation(err) {
			return model.Friendship{}, false, apperror.InvalidArgument("invalid friendship")
		}
		return model.Friendship{}, false, err
	}
	return friendship.Clone(), created, nil
}

func (r *PostgresRepository) DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var row postgresFriendshipRow
		if err := session.QueryRowCtx(ctx, &row, `
update friendships
set status = $3, updated_at = now()
where user_id = $1 and friend_id = $2 and status = $4
returning user_id, friend_id, status, created_at, updated_at
`, userID, friendID, model.FriendshipStatusDeleted, model.FriendshipStatusActive); err != nil {
			return err
		}
		_, err := session.ExecCtx(ctx, `
update friendships
set status = $3, updated_at = now()
where user_id = $1 and friend_id = $2 and status = $4
`, friendID, userID, model.FriendshipStatusDeleted, model.FriendshipStatusActive)
		if err != nil {
			return err
		}
		friendship = row.friendship()
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return model.Friendship{}, false, apperror.NotFound("friendship not found")
		}
		return model.Friendship{}, false, err
	}
	return friendship.Clone(), true, nil
}

func (r *PostgresRepository) ListFriends(ctx context.Context, userID string) ([]model.Friendship, error) {
	var rows []postgresFriendshipRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select user_id, friend_id, status, created_at, updated_at
from friendships
where user_id = $1 and status = $2
order by friend_id asc
`, userID, model.FriendshipStatusActive)
	if err != nil {
		return nil, err
	}

	friendships := make([]model.Friendship, 0, len(rows))
	for _, row := range rows {
		friendships = append(friendships, row.friendship())
	}
	return friendships, nil
}

func (r *PostgresRepository) GetFriendship(ctx context.Context, userID string, friendID string) (model.Friendship, error) {
	friendship, err := queryFriendship(ctx, r.conn, userID, friendID, false)
	if err != nil {
		if isNotFound(err) {
			return model.Friendship{}, apperror.NotFound("friendship not found")
		}
		return model.Friendship{}, err
	}
	return friendship.Clone(), nil
}

func queryFriendship(ctx context.Context, session sqlx.Session, userID string, friendID string, forUpdate bool) (model.Friendship, error) {
	query := `
select user_id, friend_id, status, created_at, updated_at
from friendships
where user_id = $1 and friend_id = $2
`
	if forUpdate {
		query += " for update"
	}

	var row postgresFriendshipRow
	if err := session.QueryRowCtx(ctx, &row, query, userID, friendID); err != nil {
		return model.Friendship{}, err
	}
	return row.friendship(), nil
}

func upsertActiveFriendship(ctx context.Context, session sqlx.Session, userID string, friendID string) (model.Friendship, error) {
	var row postgresFriendshipRow
	if err := session.QueryRowCtx(ctx, &row, `
insert into friendships (user_id, friend_id, status)
values ($1, $2, $3)
on conflict (user_id, friend_id) do update
set status = excluded.status,
    created_at = now(),
    updated_at = now()
returning user_id, friend_id, status, created_at, updated_at
`, userID, friendID, model.FriendshipStatusActive); err != nil {
		return model.Friendship{}, err
	}
	return row.friendship(), nil
}

func (r postgresUserRow) user() model.User {
	return model.User{
		UserID:      r.UserID,
		Identifier:  r.Identifier,
		DisplayName: r.DisplayName,
		Name:        r.Name,
		Gender:      r.Gender,
		Age:         r.Age,
		Region:      r.Region,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (r postgresFriendshipRow) friendship() model.Friendship {
	return model.Friendship{
		UserID:    r.UserID,
		FriendID:  r.FriendID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func itoa(value int) string {
	switch {
	case value == 0:
		return "0"
	case value < 0:
		return "-" + itoa(-value)
	default:
		var buf [20]byte
		i := len(buf)
		for value > 0 {
			i--
			buf[i] = byte('0' + value%10)
			value /= 10
		}
		return string(buf[i:])
	}
}
