package logic

import (
	"context"
	"strconv"
	"testing"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
)

func TestLoginResponseIncludesDurableAvatarProfileFields(t *testing.T) {
	ctx := context.Background()
	users := newAuthProfileClient()
	repo := authrepo.NewMemoryRepository()
	authLogic := NewAuthLogic(repo, users, NewPasswordHasher(), token.NewHMACTokenManager("unit-test-secret", time.Hour))

	registered, err := authLogic.Register(ctx, RegisterRequest{
		Identifier:  "alice_avatar",
		Password:    "test-password",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	users.profiles[registered.UserID] = useradapter.UserProfile{
		UserID:        registered.UserID,
		Identifier:    "alice_avatar",
		DisplayName:   "Alice Chen",
		Name:          "Alice Chen",
		Gender:        "female",
		BirthDate:     "1996-05-02",
		Region:        "Shanghai",
		AccountType:   "user",
		AvatarMediaID: "med_avatar_1",
		AvatarURL:     "/media/avatars/med_avatar_1",
	}

	loggedIn, err := authLogic.Login(ctx, LoginRequest{
		Identifier: "alice_avatar",
		Password:   "test-password",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if loggedIn.DisplayName != "Alice Chen" {
		t.Fatalf("display_name = %q, want Alice Chen", loggedIn.DisplayName)
	}
	if loggedIn.AvatarMediaID != "med_avatar_1" {
		t.Fatalf("avatar_media_id = %q, want med_avatar_1", loggedIn.AvatarMediaID)
	}
	if loggedIn.AvatarURL != "/media/avatars/med_avatar_1" {
		t.Fatalf("avatar_url = %q, want durable profile URL", loggedIn.AvatarURL)
	}
}

type authProfileClient struct {
	nextID       int
	identifierID map[string]string
	profiles     map[string]useradapter.UserProfile
}

func newAuthProfileClient() *authProfileClient {
	return &authProfileClient{
		nextID:       1000,
		identifierID: make(map[string]string),
		profiles:     make(map[string]useradapter.UserProfile),
	}
}

func (c *authProfileClient) ExistsByIdentifier(_ context.Context, identifier string) (useradapter.ExistsResult, error) {
	_, exists := c.identifierID[identifier]
	return useradapter.ExistsResult{Identifier: identifier, Exists: exists}, nil
}

func (c *authProfileClient) CreateUser(_ context.Context, req useradapter.CreateUserRequest) (useradapter.UserProfile, error) {
	c.nextID++
	userID := "auth_user_" + strconv.Itoa(c.nextID)
	profile := useradapter.UserProfile{
		UserID:      userID,
		Identifier:  req.Identifier,
		DisplayName: req.DisplayName,
		Name:        req.DisplayName,
		AccountType: "user",
	}
	c.identifierID[req.Identifier] = userID
	c.profiles[userID] = profile
	return profile, nil
}

func (c *authProfileClient) GetUserByID(_ context.Context, userID string) (useradapter.UserProfile, error) {
	return c.profiles[userID], nil
}
