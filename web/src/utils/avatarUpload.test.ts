import { describe, expect, it, vi } from 'vitest';
import type { MediaApi } from '../api/media';
import type { UserApi, UserProfile, UserProfilePatch } from '../api/user';
import { AVATAR_MAX_BYTES, prepareAvatarFile, uploadAvatarForProfile } from './avatarUpload';

const profile: UserProfile = {
  user_id: '1001',
  identifier: 'alice_001',
  display_name: 'Alice Chen',
  name: 'Alice Chen',
  gender: 'female',
  birth_date: '1996-05-02',
  region: 'Shanghai',
  avatar_media_id: 'med_avatar_1',
  avatar_url: 'https://storage.test/avatar/alice.png',
  avatar_url_expires_at: 1777550400000,
};

function fileOfSize(name: string, type: string, sizeBytes: number) {
  return new File([new Uint8Array(sizeBytes)], name, { type });
}

function createMediaApi(): MediaApi {
  return {
    createUploadIntent: vi.fn(async (request) => ({
      mediaId: 'med_avatar_1',
      objectKey: `objects/${request.filename}`,
      uploadUrl: 'https://storage.test/upload/avatar',
      expiresAt: 1777465000000,
    })),
    completeUpload: vi.fn(async (mediaId) => ({
      media: {
        mediaId,
        ownerUserId: '1001',
        bucket: 'agents-im-media',
        objectKey: 'objects/avatar',
        sha256: '',
        contentType: 'image/jpeg',
        sizeBytes: 1024,
        originalFilename: 'avatar.jpg',
        purpose: 'avatar',
        status: 'ready',
        createdAt: '2026-05-04T12:00:00Z',
        updatedAt: '2026-05-04T12:00:00Z',
      },
    })),
    getDownloadURL: vi.fn(async (mediaId) => ({
      mediaId,
      downloadUrl: `https://media.test/download/${mediaId}`,
      expiresAt: 1777465000000,
    })),
  };
}

function createUserApi(): UserApi {
  return {
    getCurrentUser: vi.fn(async () => profile),
    patchCurrentUser: vi.fn(async (patch: UserProfilePatch) => ({ ...profile, ...patch })),
    patchCurrentUserAvatar: vi.fn(async () => profile),
    identifierExists: vi.fn(async (identifier) => ({ identifier, exists: true })),
    getPublicProfileByIdentifier: vi.fn(async () => profile),
  };
}

describe('avatar upload helpers', () => {
  it('rejects unsupported avatar MIME before creating an upload intent', async () => {
    const mediaApi = createMediaApi();
    const userApi = createUserApi();

    await expect(
      uploadAvatarForProfile({
        file: fileOfSize('avatar.gif', 'image/gif', 1024),
        mediaApi,
        userApi,
        fetchImpl: vi.fn<typeof fetch>(),
      }),
    ).rejects.toThrow('头像仅支持 JPG、PNG 或 WebP');

    expect(mediaApi.createUploadIntent).not.toHaveBeenCalled();
    expect(userApi.patchCurrentUserAvatar).not.toHaveBeenCalled();
  });

  it('rejects avatars over the backend hard limit before upload', async () => {
    const mediaApi = createMediaApi();
    const userApi = createUserApi();

    await expect(
      uploadAvatarForProfile({
        file: fileOfSize('huge.jpg', 'image/jpeg', AVATAR_MAX_BYTES + 1),
        mediaApi,
        userApi,
        fetchImpl: vi.fn<typeof fetch>(),
      }),
    ).rejects.toThrow('头像不能超过 5 MiB');

    expect(mediaApi.createUploadIntent).not.toHaveBeenCalled();
    expect(userApi.patchCurrentUserAvatar).not.toHaveBeenCalled();
  });

  it('uses media intent, PUT, complete, and profile avatar update in order', async () => {
    const mediaApi = createMediaApi();
    const userApi = createUserApi();
    const uploadFetch = vi.fn<typeof fetch>(async () => new Response('', { status: 200 }));
    const avatar = fileOfSize('avatar.jpg', 'image/jpeg', 1024);

    const updated = await uploadAvatarForProfile({ file: avatar, mediaApi, userApi, fetchImpl: uploadFetch });

    expect(mediaApi.createUploadIntent).toHaveBeenCalledWith({
      purpose: 'avatar',
      filename: 'avatar.jpg',
      contentType: 'image/jpeg',
      sizeBytes: avatar.size,
    });
    expect(uploadFetch).toHaveBeenCalledWith(
      'https://storage.test/upload/avatar',
      expect.objectContaining({
        method: 'PUT',
        body: avatar,
        headers: { 'Content-Type': 'image/jpeg' },
      }),
    );
    expect(mediaApi.completeUpload).toHaveBeenCalledWith('med_avatar_1');
    expect(userApi.patchCurrentUserAvatar).toHaveBeenCalledWith('med_avatar_1');
    expect(vi.mocked(mediaApi.createUploadIntent).mock.invocationCallOrder[0]).toBeLessThan(uploadFetch.mock.invocationCallOrder[0]);
    expect(uploadFetch.mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(mediaApi.completeUpload).mock.invocationCallOrder[0]);
    expect(vi.mocked(mediaApi.completeUpload).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(userApi.patchCurrentUserAvatar).mock.invocationCallOrder[0],
    );
    expect(updated.avatar_url).toBe(profile.avatar_url);
  });

  it('keeps a valid small avatar unchanged when browser compression is not practical', async () => {
    const avatar = fileOfSize('avatar.png', 'image/png', 2048);

    const prepared = await prepareAvatarFile(avatar);

    expect(prepared.file).toBe(avatar);
    expect(prepared.width).toBeUndefined();
    expect(prepared.height).toBeUndefined();
  });
});
