package logic

import (
	"context"
	"strings"
	"testing"
	"time"

	sharedmodel "github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes（脱 internal/repository：media 数据走 fake model，跨域鉴权走 fake reader/checker）---

type fakeMediaModel struct {
	model.MediaObjectsModel
	rows map[string]*model.MediaObjects
}

func newFakeMediaModel(rows ...*model.MediaObjects) *fakeMediaModel {
	m := &fakeMediaModel{rows: map[string]*model.MediaObjects{}}
	for _, r := range rows {
		m.rows[r.MediaId] = r
	}
	return m
}

func (m *fakeMediaModel) FindOne(_ context.Context, mediaID string) (*model.MediaObjects, error) {
	if r, ok := m.rows[mediaID]; ok {
		copy := *r
		return &copy, nil
	}
	return nil, model.ErrNotFound
}

func (m *fakeMediaModel) CreateMediaObject(_ context.Context, data *model.MediaObjects) (*model.MediaObjects, error) {
	stored := *data
	stored.CreatedAt = time.Unix(0, 0).UTC()
	stored.UpdatedAt = stored.CreatedAt
	m.rows[stored.MediaId] = &stored
	copy := stored
	return &copy, nil
}

func (m *fakeMediaModel) UpdateStatus(_ context.Context, mediaID string, status int64) (*model.MediaObjects, error) {
	r, ok := m.rows[mediaID]
	if !ok {
		return nil, model.ErrNotFound
	}
	r.Status = status
	copy := *r
	return &copy, nil
}

type fakeAccounts struct {
	users map[string]sharedmodel.User
}

func (f fakeAccounts) GetByID(_ context.Context, accountID string) (sharedmodel.User, error) {
	if u, ok := f.users[accountID]; ok {
		return u, nil
	}
	return sharedmodel.User{}, apperror.NotFound("account not found")
}

type fakeAttachmentAccess struct {
	allow map[string]bool // userID|mediaID -> allowed
}

func (f fakeAttachmentAccess) UserCanAccessMedia(_ context.Context, userID, mediaID string) (bool, error) {
	return f.allow[userID+"|"+mediaID], nil
}

func newMemStore(infos ...objectstorage.ObjectInfo) *objectstorage.MemoryStore {
	s := objectstorage.NewMemoryStore()
	for _, info := range infos {
		s.PutObjectInfo(info)
	}
	return s
}

func wantCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", code)
	}
	if st, _ := status.FromError(err); st.Code() != code {
		t.Fatalf("expected code %s, got %s (%v)", code, st.Code(), err)
	}
}

func readyMedia(mediaID, owner string, purpose int64, contentType string, size int64) *model.MediaObjects {
	return &model.MediaObjects{
		MediaId:          mediaID,
		OwnerAccountId:   owner,
		Bucket:           "agents-im-media",
		ObjectKey:        "users/" + owner + "/media/" + mediaID + "/file",
		OriginalFilename: "file",
		ContentType:      contentType,
		SizeBytes:        size,
		Purpose:          purpose,
		Status:           model.MediaStatusReady,
	}
}

// --- CreateUploadIntent ---

func TestCreateUploadIntentSanitizesObjectKeyAndPersists(t *testing.T) {
	fm := newFakeMediaModel()
	svcCtx := &svc.ServiceContext{MediaModel: fm, Store: newMemStore(), Bucket: "agents-im-media"}

	resp, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "usr_owner",
		Purpose:     purposeMessageImage,
		Filename:    "../../client/chosen/cat photo.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   12345,
		Sha256:      strings.Repeat("a", 64),
		Width:       1080,
		Height:      720,
	})
	if err != nil {
		t.Fatalf("CreateUploadIntent: %v", err)
	}
	if !strings.HasPrefix(resp.GetMediaId(), "med_") {
		t.Fatalf("media id = %q, want med_ prefix", resp.GetMediaId())
	}
	if strings.Contains(resp.GetObjectKey(), "..") || strings.Contains(resp.GetObjectKey(), "client/chosen") {
		t.Fatalf("object key leaks client path: %q", resp.GetObjectKey())
	}
	stored, ok := fm.rows[resp.GetMediaId()]
	if !ok || stored.Status != model.MediaStatusPending || stored.Purpose != model.MediaPurposeMessageImage {
		t.Fatalf("stored object not pending message_image: %+v", stored)
	}
	if !strings.Contains(stored.Metadata, "sha256") {
		t.Fatalf("metadata missing sha256: %q", stored.Metadata)
	}
}

func TestCreateUploadIntentRejectsInvalidPurpose(t *testing.T) {
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(), Store: newMemStore(), Bucket: "agents-im-media"}
	_, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "usr_owner",
		Purpose:     "agent_skill",
		Filename:    "a.bin",
		ContentType: "application/octet-stream",
		SizeBytes:   10,
	})
	wantCode(t, err, codes.InvalidArgument)
}

// --- CompleteUpload ---

func TestCompleteUploadMarksReadyWhenObjectMatches(t *testing.T) {
	pending := &model.MediaObjects{
		MediaId:        "med_pending",
		OwnerAccountId: "usr_owner",
		Bucket:         "agents-im-media",
		ObjectKey:      "users/usr_owner/media/med_pending/cat.jpg",
		ContentType:    "image/jpeg",
		SizeBytes:      1024,
		Purpose:        model.MediaPurposeMessageImage,
		Status:         model.MediaStatusPending,
	}
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 1024})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}

	resp, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{
		OwnerUserId: "usr_owner",
		MediaId:     "med_pending",
	})
	if err != nil {
		t.Fatalf("CompleteUpload: %v", err)
	}
	if resp.GetMedia().GetStatus() != statusReady {
		t.Fatalf("status = %q, want ready", resp.GetMedia().GetStatus())
	}
}

func TestCompleteUploadRejectsSizeMismatch(t *testing.T) {
	pending := &model.MediaObjects{
		MediaId: "med_pending", OwnerAccountId: "usr_owner", Bucket: "agents-im-media",
		ObjectKey: "users/usr_owner/media/med_pending/cat.jpg", ContentType: "image/jpeg",
		SizeBytes: 1024, Purpose: model.MediaPurposeMessageImage, Status: model.MediaStatusPending,
	}
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 9999})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	_, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: "usr_owner", MediaId: "med_pending"})
	wantCode(t, err, codes.InvalidArgument)
}

// --- GetDownloadURL 鉴权 ---

func TestGetDownloadURLOwnerAllowed(t *testing.T) {
	m := readyMedia("med_dl", "usr_owner", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore()}
	resp, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "usr_owner", MediaId: "med_dl"})
	if err != nil {
		t.Fatalf("GetDownloadURL: %v", err)
	}
	if resp.GetDownloadUrl() == "" {
		t.Fatal("expected non-empty download url")
	}
}

func TestGetDownloadURLNonOwnerForbidden(t *testing.T) {
	m := readyMedia("med_dl", "usr_owner", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts:         fakeAccounts{users: map[string]sharedmodel.User{}},
		AttachmentAccess: fakeAttachmentAccess{allow: map[string]bool{}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "usr_owner", RequesterUserId: "usr_other", MediaId: "med_dl"})
	wantCode(t, err, codes.PermissionDenied)
}

func TestGetDownloadURLAdminAllowed(t *testing.T) {
	m := readyMedia("med_dl", "usr_owner", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts: fakeAccounts{users: map[string]sharedmodel.User{"usr_admin": {AccountType: sharedmodel.AccountTypeAdmin}}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "usr_owner", RequesterUserId: "usr_admin", MediaId: "med_dl"})
	if err != nil {
		t.Fatalf("admin download: %v", err)
	}
}

func TestGetDownloadURLMessageParticipantAllowed(t *testing.T) {
	m := readyMedia("med_dl", "usr_sender", model.MediaPurposeMessageImage, "image/jpeg", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts:         fakeAccounts{users: map[string]sharedmodel.User{}},
		AttachmentAccess: fakeAttachmentAccess{allow: map[string]bool{"usr_receiver|med_dl": true}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "usr_sender", RequesterUserId: "usr_receiver", MediaId: "med_dl"})
	if err != nil {
		t.Fatalf("participant download: %v", err)
	}
}

// --- GetAvatarDisplayURL ---

func TestGetAvatarDisplayURLReturnsPresignedURL(t *testing.T) {
	m := readyMedia("med_avatar", "usr_owner", model.MediaPurposeAvatar, "image/png", 128)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore()}
	resp, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: "med_avatar"})
	if err != nil {
		t.Fatalf("GetAvatarDisplayURL: %v", err)
	}
	if resp.GetMediaId() != "med_avatar" || resp.GetDownloadUrl() == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus guards that media-rpc
// emits a typed gRPC NotFound (not opaque Unknown) so media-api maps it to HTTP
// 404 instead of 500 (#390).
func TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus(t *testing.T) {
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(), Store: newMemStore()}
	_, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: "med_missing"})
	wantCode(t, err, codes.NotFound)
}

// --- 输入校验 ---

func TestValidateMediaIDComponent(t *testing.T) {
	if _, err := validateMediaIDComponent("", "media_id"); err == nil {
		t.Fatal("empty id should fail")
	}
	if _, err := validateMediaIDComponent(strings.Repeat("x", 129), "media_id"); err == nil {
		t.Fatal("over-long id should fail")
	}
	if _, err := validateMediaIDComponent("bad/slash", "media_id"); err == nil {
		t.Fatal("slash should fail")
	}
}
