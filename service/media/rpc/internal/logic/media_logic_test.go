package logic

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	sharedmodel "github.com/wujunhui99/agents_im/pkg/model"
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

func (m *fakeMediaModel) FindReadyByObjectKey(_ context.Context, objectKey string) (*model.MediaObjects, error) {
	for _, r := range m.rows {
		if r.Status == model.MediaStatusReady && r.ObjectKey == objectKey {
			copy := *r
			return &copy, nil
		}
	}
	return nil, model.ErrNotFound
}

func (m *fakeMediaModel) MarkReady(_ context.Context, mediaID int64, objectKey string, digestAlgo int64) (*model.MediaObjects, error) {
	r, ok := m.rows[mediaID]
	if !ok {
		return nil, model.ErrNotFound
	}
	r.ObjectKey = objectKey
	r.DigestAlgo = digestAlgo
	r.Status = model.MediaStatusReady
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
	// 内容寻址：返回的 object_key 是 agents_im/{sha256}，不信客户端文件名（无 path / 原文件名）。
	if resp.GetObjectKey() != finalObjectKey(strings.Repeat("a", 64)) {
		t.Fatalf("object key = %q, want content-addressed agents_im/{sha256}", resp.GetObjectKey())
	}
	if resp.GetAlreadyComplete() || resp.GetUploadUrl() == "" {
		t.Fatalf("miss should return an upload url and already_complete=false: %+v", resp)
	}
	stored, ok := fm.rows[mediaID]
	if !ok || stored.Status != model.MediaStatusPending || stored.Purpose != model.MediaPurposeMessageImage {
		t.Fatalf("stored object not pending message_image: %+v", stored)
	}
	// pending 行的 object_key 先记 tmp/{media_id}/{sha256}（confirm 时再落最终 key）。
	if stored.ObjectKey != tmpObjectKey(mediaID, strings.Repeat("a", 64)) {
		t.Fatalf("stored pending object_key = %q, want tmp key", stored.ObjectKey)
	}
	if !strings.Contains(stored.Metadata, "sha256") {
		t.Fatalf("metadata missing sha256: %q", stored.Metadata)
	}
}

// TestCreateUploadIntentInstantUpload 文件级秒传：finalKey 已有 ready 行时零字节落新 ready 行，
// 返回 already_complete=true、无 upload_url（EPIC #527 §3）。
func TestCreateUploadIntentInstantUpload(t *testing.T) {
	sha := strings.Repeat("b", 64)
	finalKey := finalObjectKey(sha)
	existing := &model.MediaObjects{
		MediaId: 4242, UploaderId: "111000111000111000", Bucket: "agents-im-media",
		ObjectKey: finalKey, OriginalFilename: "first.jpg", ContentType: "image/jpeg",
		SizeBytes: 2048, Purpose: model.MediaPurposeMessageImage, Status: model.MediaStatusReady,
		DigestAlgo: model.MediaDigestAlgoSHA256,
	}
	fm := newFakeMediaModel(existing)
	svcCtx := &svc.ServiceContext{MediaModel: fm, MediaIDGen: newMediaIDGen(t), Store: newMemStore(), Bucket: "agents-im-media"}

	resp, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "323130844539310080",
		Purpose:     purposeMessageImage,
		Filename:    "reupload.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   2048,
		Sha256:      sha,
	})
	if err != nil {
		t.Fatalf("CreateUploadIntent: %v", err)
	}
	if !resp.GetAlreadyComplete() || resp.GetUploadUrl() != "" || resp.GetObjectKey() != finalKey {
		t.Fatalf("instant upload response unexpected: %+v", resp)
	}
	newID, _ := strconv.ParseInt(resp.GetMediaId(), 10, 64)
	stored, ok := fm.rows[newID]
	if !ok || stored.Status != model.MediaStatusReady || stored.ObjectKey != finalKey {
		t.Fatalf("instant upload did not persist a ready row reusing object_key: %+v", stored)
	}
	// 复用既有对象真实 size/content-type，且新行是独立 media_id（多行共享 object_key）。
	if stored.SizeBytes != existing.SizeBytes || stored.ContentType != existing.ContentType {
		t.Fatalf("instant row should reuse object size/content-type: %+v", stored)
	}
	if newID == existing.MediaId {
		t.Fatal("instant upload should allocate a new media_id, not reuse the existing row")
	}
}

// TestCreateUploadIntentRejectsMissingSha256 sha256 是内容寻址主键，必传。
func TestCreateUploadIntentRejectsMissingSha256(t *testing.T) {
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(), MediaIDGen: newMediaIDGen(t), Store: newMemStore(), Bucket: "agents-im-media"}
	_, err := NewCreateUploadIntentLogic(context.Background(), svcCtx).CreateUploadIntent(&media.CreateUploadIntentRequest{
		OwnerUserId: "323130844539310080",
		Purpose:     purposeMessageImage,
		Filename:    "a.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   10,
	})
	wantCode(t, err, codes.InvalidArgument)
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

// checksumB64 returns the base64 SHA-256 checksum (OSS HeadObject form) for a hex digest.
func checksumB64(t *testing.T, sha256Hex string) string {
	t.Helper()
	raw, err := hex.DecodeString(sha256Hex)
	if err != nil {
		t.Fatalf("decode sha256 hex: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// sha256Hex64 returns a valid 64-hex digest seeded by a single hex nibble.
func sha256Hex64(nibble string) string { return strings.Repeat(nibble, 64) }

func pendingTmpRow(t *testing.T, mediaID int64, owner, sha string, size int64) *model.MediaObjects {
	t.Helper()
	return &model.MediaObjects{
		MediaId: mediaID, UploaderId: owner, Bucket: "agents-im-media",
		ObjectKey: tmpObjectKey(mediaID, sha), ContentType: "image/jpeg",
		SizeBytes: size, Purpose: model.MediaPurposeMessageImage, Status: model.MediaStatusPending,
	}
}

func TestCompleteUploadVerifiesChecksumRenamesAndMarksReady(t *testing.T) {
	const pendingID int64 = 7003
	owner := "323130844539310080"
	sha := sha256Hex64("c")
	pending := pendingTmpRow(t, pendingID, owner, sha, 1024)
	tmpKey := pending.ObjectKey
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: tmpKey, ContentType: "image/jpeg", SizeBytes: 1024, ChecksumSHA256: checksumB64(t, sha)})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}

	resp, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{
		OwnerUserId: owner, MediaId: strconv.FormatInt(pendingID, 10),
	})
	if err != nil {
		t.Fatalf("CompleteUpload: %v", err)
	}
	if resp.GetMedia().GetStatus() != statusReady {
		t.Fatalf("status = %q, want ready", resp.GetMedia().GetStatus())
	}
	if resp.GetMedia().GetObjectKey() != finalObjectKey(sha) {
		t.Fatalf("object key = %q, want agents_im/{sha256}", resp.GetMedia().GetObjectKey())
	}
	// tmp 改名到 finalKey：final 存在、tmp 已删（copy+delete）。
	if _, err := store.StatObject(context.Background(), finalObjectKey(sha)); err != nil {
		t.Fatalf("final object missing after rename: %v", err)
	}
	if _, err := store.StatObject(context.Background(), tmpKey); err == nil {
		t.Fatal("tmp object should be removed after rename")
	}
}

// TestCompleteUploadRejectsChecksumMismatch OSS 返回的 checksum 与上报 sha256 不符 → 拒。
func TestCompleteUploadRejectsChecksumMismatch(t *testing.T) {
	const pendingID int64 = 7004
	owner := "323130844539310080"
	sha := sha256Hex64("c")
	pending := pendingTmpRow(t, pendingID, owner, sha, 1024)
	// 对象的真实 checksum 是另一个 sha（OSS 校验了，但内容 hash 不是上报值）。
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 1024, ChecksumSHA256: checksumB64(t, sha256Hex64("d"))})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	_, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: owner, MediaId: strconv.FormatInt(pendingID, 10)})
	wantCode(t, err, codes.InvalidArgument)
}

// TestCompleteUploadRejectsMissingChecksum OSS 未返回 server-side checksum → 失败（media 不回算兜底）。
func TestCompleteUploadRejectsMissingChecksum(t *testing.T) {
	const pendingID int64 = 7005
	owner := "323130844539310080"
	sha := sha256Hex64("c")
	pending := pendingTmpRow(t, pendingID, owner, sha, 1024)
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 1024}) // 无 ChecksumSHA256
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	_, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: owner, MediaId: strconv.FormatInt(pendingID, 10)})
	wantCode(t, err, codes.Internal)
}

func TestCompleteUploadRejectsSizeMismatch(t *testing.T) {
	const pendingID int64 = 7006
	owner := "323130844539310080"
	sha := sha256Hex64("c")
	pending := pendingTmpRow(t, pendingID, owner, sha, 1024)
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: pending.ObjectKey, ContentType: "image/jpeg", SizeBytes: 9999, ChecksumSHA256: checksumB64(t, sha)})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	_, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: owner, MediaId: strconv.FormatInt(pendingID, 10)})
	wantCode(t, err, codes.InvalidArgument)
}

// TestCompleteUploadIdempotentWhenAlreadyReady confirm 重试已 ready 的行直接回放，不碰 OSS。
func TestCompleteUploadIdempotentWhenAlreadyReady(t *testing.T) {
	const id int64 = 7007
	owner := "323130844539310080"
	ready := readyMedia(id, owner, model.MediaPurposeMessageImage, "image/jpeg", 1024)
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(ready), Store: newMemStore(), Bucket: "agents-im-media"}
	resp, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: owner, MediaId: strconv.FormatInt(id, 10)})
	if err != nil {
		t.Fatalf("idempotent complete: %v", err)
	}
	if resp.GetMedia().GetStatus() != statusReady {
		t.Fatalf("status = %q, want ready", resp.GetMedia().GetStatus())
	}
}

// TestCompleteUploadResumesWhenTmpAlreadyRenamed 前次 confirm 已 copy+delete 但 MarkReady 失败：
// tmp 不在、finalKey 已就位 → 重试据 finalKey 续落 ready（copy/delete 幂等）。
func TestCompleteUploadResumesWhenTmpAlreadyRenamed(t *testing.T) {
	const pendingID int64 = 7008
	owner := "323130844539310080"
	sha := sha256Hex64("c")
	pending := pendingTmpRow(t, pendingID, owner, sha, 1024)
	// 只有 finalKey，没有 tmp（模拟前次部分执行）。
	store := newMemStore(objectstorage.ObjectInfo{ObjectKey: finalObjectKey(sha), ContentType: "image/jpeg", SizeBytes: 1024, ChecksumSHA256: checksumB64(t, sha)})
	svcCtx := &svc.ServiceContext{MediaModel: newFakeMediaModel(pending), Store: store, Bucket: "agents-im-media"}
	resp, err := NewCompleteUploadLogic(context.Background(), svcCtx).CompleteUpload(&media.CompleteUploadRequest{OwnerUserId: owner, MediaId: strconv.FormatInt(pendingID, 10)})
	if err != nil {
		t.Fatalf("resume complete: %v", err)
	}
	if resp.GetMedia().GetStatus() != statusReady || resp.GetMedia().GetObjectKey() != finalObjectKey(sha) {
		t.Fatalf("resume complete did not finalize: %+v", resp.GetMedia())
	}
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

// --- tmp 清理 ---

// TestTempUploadCleanerRemovesStaleTmpOnly 只回收 tmp/ 下早于 maxAge 的孤儿；新 tmp 与非 tmp 对象保留。
func TestTempUploadCleanerRemovesStaleTmpOnly(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	store := objectstorage.NewMemoryStore()
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: "tmp/100/" + sha256Hex64("a"), SizeBytes: 1, LastModified: now.Add(-2 * time.Hour)})     // stale
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: "tmp/200/" + sha256Hex64("b"), SizeBytes: 1, LastModified: now.Add(-5 * time.Minute)})   // fresh
	store.PutObjectInfo(objectstorage.ObjectInfo{ObjectKey: finalObjectKey(sha256Hex64("c")), SizeBytes: 1, LastModified: now.Add(-48 * time.Hour)}) // not tmp

	cleaner := &TempUploadCleaner{store: store, maxAge: time.Hour, now: func() time.Time { return now }}
	removed, err := cleaner.SweepOnce(context.Background())
	if err != nil {
		t.Fatalf("SweepOnce: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1 (only the stale tmp object)", removed)
	}
	if _, err := store.StatObject(context.Background(), "tmp/100/"+sha256Hex64("a")); err == nil {
		t.Fatal("stale tmp object should be removed")
	}
	if _, err := store.StatObject(context.Background(), "tmp/200/"+sha256Hex64("b")); err != nil {
		t.Fatalf("fresh tmp object should be kept: %v", err)
	}
	if _, err := store.StatObject(context.Background(), finalObjectKey(sha256Hex64("c"))); err != nil {
		t.Fatalf("non-tmp object should be kept: %v", err)
	}
}
