package logic

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	sharedmodel "github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/media"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes（脱 internal/repository：media 数据走 fake model，跨域鉴权走 fake reader/checker）---
// media_id 为雪花 bigint（EPIC #527 §1），wire 上以十进制字符串承载（ADR #529）；故 fake model
// 按 int64 主键存，请求/断言用 strconv 在 int64 与十进制串之间转。

type fakeMediaModel struct {
	model.MediaObjectsModel
	rows map[int64]*model.MediaObjects
}

func newFakeMediaModel(rows ...*model.MediaObjects) *fakeMediaModel {
	m := &fakeMediaModel{rows: map[int64]*model.MediaObjects{}}
	for _, r := range rows {
		m.rows[r.MediaId] = r
	}
	return m
}

func (m *fakeMediaModel) FindOne(_ context.Context, mediaID int64) (*model.MediaObjects, error) {
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

func (m *fakeMediaModel) UpdateStatus(_ context.Context, mediaID int64, status int64) (*model.MediaObjects, error) {
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
	allow map[string]bool // userID|mediaID(十进制串) -> allowed
}

func (f fakeAttachmentAccess) UserCanAccessMedia(_ context.Context, userID, mediaID string) (bool, error) {
	return f.allow[userID+"|"+mediaID], nil
}

// newMediaIDGen 构造测试用 media_id 生成器（HintBits=0、单机 machine 0），与 svc 默认布局一致。
func newMediaIDGen(t *testing.T) *idgen.RoutedFlake {
	t.Helper()
	gen, err := idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{HintBits: 0, MachineBits: 10, MachineID: 0})
	if err != nil {
		t.Fatalf("build media id generator: %v", err)
	}
	return gen
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

func readyMedia(mediaID int64, owner string, purpose int64, contentType string, size int64) *model.MediaObjects {
	return &model.MediaObjects{
		MediaId:          mediaID,
		UploaderId:       owner,
		Bucket:           "agents-im-media",
		ObjectKey:        "media/" + owner + "/" + strconv.FormatInt(mediaID, 10) + "/file",
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
	svcCtx := &svc.ServiceContext{MediaModel: fm, MediaIDGen: newMediaIDGen(t), Store: newMemStore(), Bucket: "agents-im-media"}

	resp, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "323130844539310080",
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
	mediaID, err := strconv.ParseInt(resp.GetMediaId(), 10, 64)
	if err != nil || mediaID <= 0 {
		t.Fatalf("media id = %q, want positive decimal snowflake", resp.GetMediaId())
	}
	// object_key 不信客户端文件名：既不含 ".."，也不含原始 path / 文件名。
	if strings.Contains(resp.GetObjectKey(), "..") || strings.Contains(resp.GetObjectKey(), "client/chosen") || strings.Contains(resp.GetObjectKey(), "cat") {
		t.Fatalf("object key leaks client path/filename: %q", resp.GetObjectKey())
	}
	stored, ok := fm.rows[mediaID]
	if !ok || stored.Status != model.MediaStatusPending || stored.Purpose != model.MediaPurposeMessageImage {
		t.Fatalf("stored object not pending message_image: %+v", stored)
	}
	if !strings.Contains(stored.Metadata, "sha256") {
		t.Fatalf("metadata missing sha256: %q", stored.Metadata)
	}
}

func TestCreateUploadIntentRejectsInvalidPurpose(t *testing.T) {
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(), MediaIDGen: newMediaIDGen(t), Store: newMemStore(), Bucket: "agents-im-media"}
	_, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "323130844539310080",
		Purpose:     "agent_skill",
		Filename:    "a.bin",
		ContentType: "application/octet-stream",
		SizeBytes:   10,
	})
	wantCode(t, err, codes.InvalidArgument)
}

// --- CompleteUpload ---

func TestCompleteUploadMarksReadyWhenObjectMatches(t *testing.T) {
	const pendingID int64 = 7003
	pending := &model.MediaObjects{
		MediaId:     pendingID,
		UploaderId:  "323130844539310080",
		Bucket:      "agents-im-media",
		ObjectKey:   "media/323130844539310080/7003/cat.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
		Purpose:     model.MediaPurposeMessageImage,
		Status:      model.MediaStatusPending,
	}
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 1024})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}

	resp, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{
		OwnerUserId: "323130844539310080",
		MediaId:     strconv.FormatInt(pendingID, 10),
	})
	if err != nil {
		t.Fatalf("CompleteUpload: %v", err)
	}
	if resp.GetMedia().GetStatus() != statusReady {
		t.Fatalf("status = %q, want ready", resp.GetMedia().GetStatus())
	}
}

func TestCompleteUploadRejectsSizeMismatch(t *testing.T) {
	const pendingID int64 = 7003
	pending := &model.MediaObjects{
		MediaId: pendingID, UploaderId: "323130844539310080", Bucket: "agents-im-media",
		ObjectKey: "media/323130844539310080/7003/cat.jpg", ContentType: "image/jpeg",
		SizeBytes: 1024, Purpose: model.MediaPurposeMessageImage, Status: model.MediaStatusPending,
	}
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 9999})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	_, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: "323130844539310080", MediaId: strconv.FormatInt(pendingID, 10)})
	wantCode(t, err, codes.InvalidArgument)
}

// --- GetDownloadURL 鉴权 ---

func TestGetDownloadURLOwnerAllowed(t *testing.T) {
	const dlID int64 = 7001
	m := readyMedia(dlID, "323130844539310080", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore()}
	resp, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "323130844539310080", MediaId: strconv.FormatInt(dlID, 10)})
	if err != nil {
		t.Fatalf("GetDownloadURL: %v", err)
	}
	if resp.GetDownloadUrl() == "" {
		t.Fatal("expected non-empty download url")
	}
}

func TestGetDownloadURLNonOwnerForbidden(t *testing.T) {
	const dlID int64 = 7001
	m := readyMedia(dlID, "323130844539310080", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts:         fakeAccounts{users: map[string]sharedmodel.User{}},
		AttachmentAccess: fakeAttachmentAccess{allow: map[string]bool{}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "323130844539310080", RequesterUserId: "999000111222333444", MediaId: strconv.FormatInt(dlID, 10)})
	wantCode(t, err, codes.PermissionDenied)
}

func TestGetDownloadURLAdminAllowed(t *testing.T) {
	const dlID int64 = 7001
	m := readyMedia(dlID, "323130844539310080", model.MediaPurposeMessageFile, "application/pdf", 2048)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts: fakeAccounts{users: map[string]sharedmodel.User{"999000111222333444": {AccountType: sharedmodel.AccountTypeAdmin}}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "323130844539310080", RequesterUserId: "999000111222333444", MediaId: strconv.FormatInt(dlID, 10)})
	if err != nil {
		t.Fatalf("admin download: %v", err)
	}
}

func TestGetDownloadURLMessageParticipantAllowed(t *testing.T) {
	const dlID int64 = 7001
	m := readyMedia(dlID, "323130844539310080", model.MediaPurposeMessageImage, "image/jpeg", 2048)
	receiver := "999000111222333444"
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore(),
		Accounts:         fakeAccounts{users: map[string]sharedmodel.User{}},
		AttachmentAccess: fakeAttachmentAccess{allow: map[string]bool{receiver + "|" + strconv.FormatInt(dlID, 10): true}},
	}
	_, err := NewGetDownloadURLLogic(context.Background(), svcCtx).GetDownloadURL(&media.GetDownloadURLRequest{OwnerUserId: "323130844539310080", RequesterUserId: receiver, MediaId: strconv.FormatInt(dlID, 10)})
	if err != nil {
		t.Fatalf("participant download: %v", err)
	}
}

// --- GetAvatarDisplayURL ---

func TestGetAvatarDisplayURLReturnsPresignedURL(t *testing.T) {
	const avatarID int64 = 7002
	m := readyMedia(avatarID, "323130844539310080", model.MediaPurposeAvatar, "image/png", 128)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(m), Store: newMemStore()}
	resp, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: strconv.FormatInt(avatarID, 10)})
	if err != nil {
		t.Fatalf("GetAvatarDisplayURL: %v", err)
	}
	if resp.GetMediaId() != strconv.FormatInt(avatarID, 10) || resp.GetDownloadUrl() == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus guards that media-rpc
// emits a typed gRPC NotFound (not opaque Unknown) so media-api maps it to HTTP
// 404 instead of 500 (#390). 用一个合法但不存在的雪花 id（解析通过、查不到）。
func TestGetAvatarDisplayURLMissingMediaReturnsNotFoundStatus(t *testing.T) {
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(), Store: newMemStore()}
	_, err := NewGetAvatarDisplayURLLogic(context.Background(), svcCtx).GetAvatarDisplayURL(&media.GetAvatarDisplayURLRequest{MediaId: "999999999999999"})
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

func TestParseMediaID(t *testing.T) {
	if _, err := parseMediaID(""); err == nil {
		t.Fatal("empty media_id should fail")
	}
	if _, err := parseMediaID("med_64cdcaf1c7841f7c"); err == nil {
		t.Fatal("legacy med_ id should fail (not decimal)")
	}
	if _, err := parseMediaID("0"); err == nil {
		t.Fatal("zero media_id should fail")
	}
	id, err := parseMediaID("58537781550383104")
	if err != nil || id != 58537781550383104 {
		t.Fatalf("parseMediaID snowflake: got %d, %v", id, err)
	}
}
