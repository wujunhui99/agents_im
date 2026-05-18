package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type postgresAccountProfileRow struct {
	AccountID        string    `db:"account_id"`
	Identifier       string    `db:"identifier"`
	AccountType      int16     `db:"account_type"`
	AccountCreatedAt time.Time `db:"account_created_at"`
	AccountUpdatedAt time.Time `db:"account_updated_at"`
	DisplayName      string    `db:"display_name"`
	Name             string    `db:"name"`
	Gender           int16     `db:"gender"`
	BirthDate        string    `db:"birth_date"`
	Region           string    `db:"region"`
	AvatarMediaID    string    `db:"avatar_media_id"`
	AvatarURL        string    `db:"avatar_url"`
	ProfileCreatedAt time.Time `db:"profile_created_at"`
	ProfileUpdatedAt time.Time `db:"profile_updated_at"`
}

type postgresFriendshipRow struct {
	UserID    string    `db:"account_id"`
	FriendID  string    `db:"friend_account_id"`
	Status    int16     `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r *PostgresRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	accountType, ok := model.NormalizeAccountType(string(user.AccountType))
	if !ok {
		return model.User{}, apperror.InvalidArgument("account_type must be user, agent, or admin")
	}

	accountID := strings.TrimSpace(user.AccountID)
	if accountID == "" {
		accountID = strings.TrimSpace(user.UserID)
	}
	if accountID == "" {
		generated, err := idgen.NewString()
		if err != nil {
			return model.User{}, err
		}
		accountID = generated
	}

	var row postgresAccountProfileRow
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `
insert into accounts (account_id, identifier, account_type)
values ($1, $2, $3)
`, accountID, user.Identifier, accountTypeToDB(accountType)); err != nil {
			return err
		}

		if _, err := session.ExecCtx(ctx, `
insert into profiles (account_id, display_name, name, gender, birth_date, region, avatar_media_id, avatar_url)
values ($1, $2, $3, $4, nullif($5, '')::date, $6, $7, $8)
`, accountID, user.DisplayName, user.Name, genderToDB(user.Gender), user.BirthDate, user.Region, strings.TrimSpace(user.AvatarMediaID), strings.TrimSpace(user.AvatarURL)); err != nil {
			return err
		}

		return session.QueryRowCtx(ctx, &row, accountProfileByIDQuery, accountID)
	})
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return model.User{}, apperror.AlreadyExists("identifier already exists")
		}
		if isPostgresCheckViolation(err) {
			return model.User{}, apperror.InvalidArgument("invalid account profile or account_type")
		}
		return model.User{}, err
	}

	return row.user(), nil
}

func (r *PostgresRepository) GetByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	var row postgresAccountProfileRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select
  a.account_id, a.identifier, a.account_type,
  a.created_at as account_created_at, a.updated_at as account_updated_at,
  p.display_name, p.name, p.gender, coalesce(p.birth_date::text, '') as birth_date, p.region, p.avatar_media_id, p.avatar_url,
  p.created_at as profile_created_at, p.updated_at as profile_updated_at
from accounts a
join profiles p on p.account_id = a.account_id
where a.identifier = $1
`, identifier)
	if err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("account not found")
		}
		return model.User{}, err
	}
	return row.user(), nil
}

func (r *PostgresRepository) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	var exists bool
	err := r.conn.QueryRowCtx(ctx, &exists, `
select exists(select 1 from accounts where identifier = $1)
`, identifier)
	return exists, err
}

func (r *PostgresRepository) SearchAccounts(ctx context.Context, filter AccountSearchFilter) ([]model.User, error) {
	limit := normalizeAdminLimit(filter.Limit, 20, 100)
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	like := "%" + query + "%"
	var rows []postgresAccountProfileRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select
  a.account_id, a.identifier, a.account_type,
  a.created_at as account_created_at, a.updated_at as account_updated_at,
  p.display_name, p.name, p.gender, coalesce(p.birth_date::text, '') as birth_date, p.region, p.avatar_media_id, p.avatar_url,
  p.created_at as profile_created_at, p.updated_at as profile_updated_at
from accounts a
join profiles p on p.account_id = a.account_id
where $1 = ''
   or lower(a.account_id) like $2
   or lower(a.identifier) like $2
   or lower(p.display_name) like $2
   or lower(p.name) like $2
order by a.created_at desc, a.account_id asc
limit $3
`, query, like, limit)
	if err != nil {
		return nil, err
	}

	users := make([]model.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.user())
	}
	return users, nil
}

func (r *PostgresRepository) CountAccounts(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn.QueryRowCtx(ctx, &count, `select count(*) from accounts`)
	return count, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, userID string) (model.User, error) {
	var row postgresAccountProfileRow
	err := r.conn.QueryRowCtx(ctx, &row, accountProfileByIDQuery, userID)
	if err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("account not found")
		}
		return model.User{}, err
	}
	return row.user(), nil
}

const accountProfileByIDQuery = `
select
  a.account_id, a.identifier, a.account_type,
  a.created_at as account_created_at, a.updated_at as account_updated_at,
  p.display_name, p.name, p.gender, coalesce(p.birth_date::text, '') as birth_date, p.region, p.avatar_media_id, p.avatar_url,
  p.created_at as profile_created_at, p.updated_at as profile_updated_at
from accounts a
join profiles p on p.account_id = a.account_id
where a.account_id = $1
`

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
		addSetter("gender", genderToDB(*patch.Gender))
	}
	if patch.BirthDate != nil {
		addSetter("birth_date", sql.NullString{String: *patch.BirthDate, Valid: strings.TrimSpace(*patch.BirthDate) != ""})
	}
	if patch.Region != nil {
		addSetter("region", *patch.Region)
	}
	if len(setters) == 0 {
		return r.GetByID(ctx, userID)
	}

	args = append(args, userID)
	query := `
update profiles
set ` + strings.Join(setters, ", ") + `, updated_at = now()
where account_id = $` + itoa(len(args)) + `
returning account_id
`
	var accountID string
	if err := r.conn.QueryRowCtx(ctx, &accountID, query, args...); err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("account not found")
		}
		if isPostgresCheckViolation(err) {
			return model.User{}, apperror.InvalidArgument("invalid account profile")
		}
		return model.User{}, err
	}
	return r.GetByID(ctx, accountID)
}

func (r *PostgresRepository) UpdateAvatar(ctx context.Context, userID string, avatarMediaID string, avatarURL string) (model.User, error) {
	var accountID string
	if err := r.conn.QueryRowCtx(ctx, &accountID, `
update profiles
set avatar_media_id = $2, avatar_url = $3, updated_at = now()
where account_id = $1
returning account_id
`, userID, strings.TrimSpace(avatarMediaID), strings.TrimSpace(avatarURL)); err != nil {
		if isNotFound(err) {
			return model.User{}, apperror.NotFound("account not found")
		}
		return model.User{}, err
	}
	return r.GetByID(ctx, accountID)
}

func (r *PostgresRepository) AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	created := true
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		existing, err := queryFriendship(ctx, session, userID, friendID, true)
		if err == nil && (existing.Status == model.FriendshipStatusAccepted || existing.Status == model.FriendshipStatusPending) {
			friendship = existing
			created = false
			return nil
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		reverse, reverseErr := queryFriendship(ctx, session, friendID, userID, true)
		if reverseErr != nil && !isNotFound(reverseErr) {
			return reverseErr
		}
		if reverseErr == nil && reverse.Status == model.FriendshipStatusPending {
			row, err := upsertFriendshipWithStatus(ctx, session, userID, friendID, model.FriendshipStatusAccepted)
			if err != nil {
				return err
			}
			if _, err := upsertFriendshipWithStatus(ctx, session, friendID, userID, model.FriendshipStatusAccepted); err != nil {
				return err
			}
			friendship = row
			return nil
		}

		row, err := upsertFriendshipWithStatus(ctx, session, userID, friendID, model.FriendshipStatusPending)
		if err != nil {
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

func (r *PostgresRepository) AcceptFriendRequest(ctx context.Context, userID string, requesterID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		incoming, err := queryFriendship(ctx, session, requesterID, userID, true)
		if err != nil {
			return err
		}
		if incoming.Status != model.FriendshipStatusPending {
			return apperror.NotFound("friend request not found")
		}
		if _, err := upsertFriendshipWithStatus(ctx, session, requesterID, userID, model.FriendshipStatusAccepted); err != nil {
			return err
		}
		row, err := upsertFriendshipWithStatus(ctx, session, userID, requesterID, model.FriendshipStatusAccepted)
		if err != nil {
			return err
		}
		friendship = row
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return model.Friendship{}, false, apperror.NotFound("friend request not found")
		}
		return model.Friendship{}, false, err
	}
	return friendship.Clone(), true, nil
}

func (r *PostgresRepository) RejectFriendRequest(ctx context.Context, userID string, requesterID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		incoming, err := queryFriendship(ctx, session, requesterID, userID, true)
		if err != nil {
			return err
		}
		if incoming.Status != model.FriendshipStatusPending {
			return apperror.NotFound("friend request not found")
		}
		if _, err := upsertFriendshipWithStatus(ctx, session, requesterID, userID, model.FriendshipStatusRejected); err != nil {
			return err
		}
		row, err := upsertFriendshipWithStatus(ctx, session, userID, requesterID, model.FriendshipStatusRejected)
		if err != nil {
			return err
		}
		friendship = row
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return model.Friendship{}, false, apperror.NotFound("friend request not found")
		}
		return model.Friendship{}, false, err
	}
	return friendship.Clone(), true, nil
}

func (r *PostgresRepository) DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	var friendship model.Friendship
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var row postgresFriendshipRow
		if err := session.QueryRowCtx(ctx, &row, `
update friendships
set status = $3, updated_at = now()
where account_id = $1 and friend_account_id = $2 and status = $4
returning account_id, friend_account_id, status, created_at, updated_at
`, userID, friendID, friendshipStatusToDB(model.FriendshipStatusDeleted), friendshipStatusToDB(model.FriendshipStatusAccepted)); err != nil {
			return err
		}
		_, err := session.ExecCtx(ctx, `
update friendships
set status = $3, updated_at = now()
where account_id = $1 and friend_account_id = $2 and status = $4
`, friendID, userID, friendshipStatusToDB(model.FriendshipStatusDeleted), friendshipStatusToDB(model.FriendshipStatusAccepted))
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
select account_id, friend_account_id, status, created_at, updated_at
from friendships
where account_id = $1 and status = $2
order by friend_account_id asc
`, userID, friendshipStatusToDB(model.FriendshipStatusAccepted))
	if err != nil {
		return nil, err
	}

	friendships := make([]model.Friendship, 0, len(rows))
	for _, row := range rows {
		friendships = append(friendships, row.friendship())
	}
	return friendships, nil
}

func (r *PostgresRepository) ListIncomingFriendRequests(ctx context.Context, userID string) ([]model.Friendship, error) {
	var rows []postgresFriendshipRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select account_id, friend_account_id, status, created_at, updated_at
from friendships
where friend_account_id = $1 and status = $2
order by account_id asc
`, userID, friendshipStatusToDB(model.FriendshipStatusPending))
	if err != nil {
		return nil, err
	}

	friendships := make([]model.Friendship, 0, len(rows))
	for _, row := range rows {
		friendships = append(friendships, row.friendship())
	}
	return friendships, nil
}

func (r *PostgresRepository) ListOutgoingFriendRequests(ctx context.Context, userID string) ([]model.Friendship, error) {
	var rows []postgresFriendshipRow
	err := r.conn.QueryRowsCtx(ctx, &rows, `
select account_id, friend_account_id, status, created_at, updated_at
from friendships
where account_id = $1 and status = $2
order by friend_account_id asc
`, userID, friendshipStatusToDB(model.FriendshipStatusPending))
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
	if err == nil {
		return friendship.Clone(), nil
	}
	if !isNotFound(err) {
		return model.Friendship{}, err
	}

	reverse, reverseErr := queryFriendship(ctx, r.conn, friendID, userID, false)
	if reverseErr == nil && reverse.Status == model.FriendshipStatusPending {
		return model.Friendship{
			UserID:    userID,
			FriendID:  friendID,
			Status:    model.FriendshipStatusPending,
			CreatedAt: reverse.CreatedAt,
			UpdatedAt: reverse.UpdatedAt,
		}, nil
	}
	if reverseErr != nil && !isNotFound(reverseErr) {
		return model.Friendship{}, reverseErr
	}
	return model.Friendship{}, apperror.NotFound("friendship not found")
}

func queryFriendship(ctx context.Context, session sqlx.Session, userID string, friendID string, forUpdate bool) (model.Friendship, error) {
	query := `
select account_id, friend_account_id, status, created_at, updated_at
from friendships
where account_id = $1 and friend_account_id = $2
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

func upsertFriendshipWithStatus(ctx context.Context, session sqlx.Session, userID string, friendID string, status string) (model.Friendship, error) {
	var row postgresFriendshipRow
	if err := session.QueryRowCtx(ctx, &row, `
insert into friendships (account_id, friend_account_id, status)
values ($1, $2, $3)
on conflict (account_id, friend_account_id) do update
set status = excluded.status,
    created_at = now(),
    updated_at = now()
returning account_id, friend_account_id, status, created_at, updated_at
`, userID, friendID, friendshipStatusToDB(status)); err != nil {
		return model.Friendship{}, err
	}
	return row.friendship(), nil
}

func (r postgresAccountProfileRow) user() model.User {
	accountType := accountTypeFromDB(r.AccountType)
	return model.NewAccountProfile(
		model.Account{
			AccountID:   r.AccountID,
			Identifier:  r.Identifier,
			AccountType: accountType,
			CreatedAt:   r.AccountCreatedAt,
			UpdatedAt:   r.AccountUpdatedAt,
		},
		model.Profile{
			AccountID:     r.AccountID,
			DisplayName:   r.DisplayName,
			Name:          r.Name,
			Gender:        genderFromDB(r.Gender),
			BirthDate:     r.BirthDate,
			Region:        r.Region,
			AvatarMediaID: r.AvatarMediaID,
			AvatarURL:     r.AvatarURL,
			CreatedAt:     r.ProfileCreatedAt,
			UpdatedAt:     r.ProfileUpdatedAt,
		},
	)
}

func (r postgresFriendshipRow) friendship() model.Friendship {
	return model.Friendship{
		UserID:    r.UserID,
		FriendID:  r.FriendID,
		Status:    friendshipStatusFromDB(r.Status),
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
