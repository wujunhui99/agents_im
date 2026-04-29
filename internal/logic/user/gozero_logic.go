package user

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateUserLogic) CreateUser(req *types.CreateUserReq) (*types.UserResp, error) {
	profile, err := l.svcCtx.UserLogic.CreateUser(l.ctx, business.CreateUserRequest{
		Identifier:  req.Identifier,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Gender:      req.Gender,
		Age:         req.Age,
		Region:      req.Region,
	})
	if err != nil {
		return nil, err
	}
	return userResp(profile), nil
}

type ExistsUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExistsUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExistsUserLogic {
	return &ExistsUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ExistsUserLogic) ExistsUser(req *types.ExistsReq) (*types.ExistsResp, error) {
	result, err := l.svcCtx.UserLogic.ExistsByIdentifier(l.ctx, business.ExistsByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return &types.ExistsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ExistsData{
			Identifier: result.Identifier,
			Exists:     result.Exists,
		},
	}, nil
}

type GetMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMeLogic {
	return &GetMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetMeLogic) GetMe(req *types.GetMeReq) (*types.UserResp, error) {
	userID, err := currentUserID(req.CurrentUserID)
	if err != nil {
		return nil, err
	}

	profile, err := l.svcCtx.UserLogic.GetUserByID(l.ctx, business.GetUserByIDRequest{UserID: userID})
	if err != nil {
		return nil, err
	}
	return userResp(profile), nil
}

type GetUserByIdentifierLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserByIdentifierLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIdentifierLogic {
	return &GetUserByIdentifierLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserByIdentifierLogic) GetUserByIdentifier(req *types.GetUserByIdentifierReq) (*types.UserResp, error) {
	profile, err := l.svcCtx.UserLogic.GetUserByIdentifier(l.ctx, business.GetUserByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return userResp(profile), nil
}

type UpdateMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMeLogic {
	return &UpdateMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeLogic) UpdateMe(req *types.UpdateMeReq) (*types.UserResp, error) {
	userID, err := currentUserID(req.CurrentUserID)
	if err != nil {
		return nil, err
	}

	profile, err := l.svcCtx.UserLogic.UpdateUserProfile(l.ctx, business.UpdateUserProfileRequest{
		UserID:      userID,
		DisplayName: optionalString(req.DisplayName),
		Name:        optionalString(req.Name),
		Gender:      optionalString(req.Gender),
		Age:         optionalAge(req.Age),
		Region:      optionalString(req.Region),
	})
	if err != nil {
		return nil, err
	}
	return userResp(profile), nil
}

func currentUserID(value string) (string, error) {
	userID := strings.TrimSpace(value)
	if userID == "" {
		return "", apperror.Unauthenticated("X-User-Id header is required")
	}
	return userID, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalAge(value int32) *int32 {
	if value == 0 {
		return nil
	}
	return &value
}

func userResp(profile business.UserProfile) *types.UserResp {
	return &types.UserResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.User{
			UserID:      profile.UserID,
			Identifier:  profile.Identifier,
			DisplayName: profile.DisplayName,
			Name:        profile.Name,
			Gender:      profile.Gender,
			Age:         profile.Age,
			Region:      profile.Region,
			CreatedAt:   profile.CreatedAt,
			UpdatedAt:   profile.UpdatedAt,
		},
	}
}
