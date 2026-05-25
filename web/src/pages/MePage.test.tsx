import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { UserProfile } from '../api/user';
import { MePage } from './MePage';

const profile: UserProfile = {
  user_id: '1001',
  identifier: 'alice_001',
  display_name: 'Alice Chen',
  name: 'Alice Chen',
  gender: 'female',
  birth_date: '1996-05-02',
  region: 'Shanghai',
  account_type: 'user',
  avatar_media_id: 'med_avatar_1',
  avatar_url: 'https://storage.test/avatar/alice.png',
  avatar_url_expires_at: 1777550400000,
};

describe('MePage', () => {
  it('shows profile labels without exposing the internal user id', () => {
    render(<MePage profile={profile} onUpdateProfile={vi.fn()} onUploadAvatar={vi.fn()} />);

    const details = screen.getByRole('region', { name: '个人资料详情' });
    expect(within(details).queryByText('user_id')).not.toBeInTheDocument();
    expect(within(details).queryByText('1001')).not.toBeInTheDocument();
    expect(within(details).getByText('alice_001')).toBeInTheDocument();
    expect(within(details).getByText('Alice Chen')).toBeInTheDocument();
    expect(within(details).getByText('用户')).toBeInTheDocument();
  });

  it('keeps Me-scoped menu entries and excludes moments', () => {
    render(<MePage profile={profile} onUpdateProfile={vi.fn()} onUploadAvatar={vi.fn()} />);

    expect(screen.getByText('服务')).toBeInTheDocument();
    expect(screen.getByText('收藏')).toBeInTheDocument();
    expect(screen.getByText('设置')).toBeInTheDocument();
    expect(screen.queryByText('朋友圈')).not.toBeInTheDocument();
  });

  it('renders the current avatar image and uploads a selected replacement', async () => {
    const user = userEvent.setup();
    const updatedProfile = {
      ...profile,
      avatar_media_id: 'med_avatar_2',
      avatar_url: 'https://storage.test/avatar/alice-new.png',
      avatar_url_expires_at: 1777554000000,
    };
    const onUploadAvatar = vi.fn(async () => updatedProfile);
    const onAvatarUpdated = vi.fn();

    render(
      <MePage profile={profile} onUpdateProfile={vi.fn()} onUploadAvatar={onUploadAvatar} onAvatarUpdated={onAvatarUpdated} />,
    );

    expect(screen.getByRole('img', { name: 'Alice Chen 头像' })).toHaveAttribute('src', profile.avatar_url);

    const avatarFile = new File([new Uint8Array(1024)], 'avatar.jpg', { type: 'image/jpeg' });
    await user.upload(screen.getByLabelText('上传头像'), avatarFile);

    await waitFor(() => expect(onUploadAvatar).toHaveBeenCalledWith(avatarFile));
    expect(onAvatarUpdated).toHaveBeenCalledWith(updatedProfile);
    expect(await screen.findByRole('status')).toHaveTextContent('头像已更新');
  });

  it('shows a Chinese error when avatar upload fails', async () => {
    const user = userEvent.setup();
    const onUploadAvatar = vi.fn(async () => {
      throw new Error('object storage unavailable');
    });

    render(<MePage profile={profile} onUpdateProfile={vi.fn()} onUploadAvatar={onUploadAvatar} />);

    await user.upload(screen.getByLabelText('上传头像'), new File([new Uint8Array(1024)], 'avatar.jpg', { type: 'image/jpeg' }));

    const status = await screen.findByRole('status');
    expect(status).toHaveTextContent('头像上传失败');
    expect(status).toHaveTextContent('object storage unavailable');
  });

  it('submits user feedback from the Me page', async () => {
    const user = userEvent.setup();
    const onSubmitFeedback = vi.fn(async () => undefined);

    render(
      <MePage profile={profile} onUpdateProfile={vi.fn()} onUploadAvatar={vi.fn()} onSubmitFeedback={onSubmitFeedback} />,
    );

    await user.click(screen.getByRole('button', { name: '意见反馈' }));
    await user.selectOptions(screen.getByLabelText('反馈类型'), 'bug');
    await user.type(screen.getByLabelText('标题'), '消息发送失败');
    await user.type(screen.getByLabelText('反馈内容'), '点击发送后没有任何响应');
    await user.type(screen.getByLabelText('联系方式（选填）'), 'alice@example.com');
    await user.click(screen.getByRole('button', { name: '提交反馈' }));

    await waitFor(() => expect(onSubmitFeedback).toHaveBeenCalledWith({
      category: 'bug',
      title: '消息发送失败',
      content: '点击发送后没有任何响应',
      contact: 'alice@example.com',
      pageUrl: expect.any(String),
      userAgent: expect.any(String),
    }));
    expect(await screen.findByRole('status')).toHaveTextContent('反馈已提交');
  });
});
