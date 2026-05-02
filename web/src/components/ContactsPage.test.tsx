import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { ContactsApi } from '../api/contacts';
import type { UserApi, UserProfile, UserProfilePatch } from '../api/user';
import ContactsPage from './ContactsPage';

const bobProfile: UserProfile = {
  user_id: '2002',
  identifier: 'bob_002',
  display_name: 'Bob Lin',
  name: 'Bob Lin',
  gender: '',
  birth_date: '',
  region: '',
};

function createUserApi(profile: UserProfile = bobProfile): UserApi {
  return {
    getCurrentUser: vi.fn(async () => profile),
    patchCurrentUser: vi.fn(async (patch: UserProfilePatch) => ({ ...profile, ...patch })),
    identifierExists: vi.fn(async (identifier) => ({ identifier, exists: true })),
    getPublicProfileByIdentifier: vi.fn(async () => profile),
  };
}

function createContactsApi(): ContactsApi {
  return {
    listFriends: vi.fn(async () => ({ friends: [] })),
    addFriend: vi.fn(async (userId) => ({
      friendship: {
        user_id: '1001',
        friend_id: userId,
        status: 'active',
        is_friend: true,
        created_at: '2026-04-29T12:00:00Z',
        updated_at: '2026-04-29T12:00:00Z',
      },
      created: true,
    })),
    deleteFriend: vi.fn(async (userId) => ({
      friendship: {
        user_id: '1001',
        friend_id: userId,
        status: 'deleted',
        is_friend: false,
        created_at: '2026-04-29T12:00:00Z',
        updated_at: '2026-04-29T12:00:00Z',
      },
      deleted: true,
    })),
  };
}

function createContactsApiWithFriend(): ContactsApi {
  const api = createContactsApi();
  api.listFriends = vi.fn(async () => ({
    friends: [
      {
        user_id: '1001',
        friend_id: '2002',
        status: 'active',
        is_friend: true,
        created_at: '2026-04-29T12:00:00Z',
        updated_at: '2026-04-29T12:00:00Z',
        friend: {
          user_id: '2002',
          identifier: 'bob_002',
          display_name: 'Cached Bob',
          name: 'Cached Bob',
        },
      },
    ],
  }));
  return api;
}

describe('ContactsPage', () => {
  it('uses the embedded friend profile when opening a chat from the friend row', async () => {
    const user = userEvent.setup();
    const userApi = createUserApi();
    const onStartChat = vi.fn();

    render(<ContactsPage userApi={userApi} contactsApi={createContactsApiWithFriend()} onStartChat={onStartChat} />);

    await user.click(await screen.findByRole('button', { name: '和 bob_002 聊天' }));

    expect(userApi.getPublicProfileByIdentifier).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(onStartChat).toHaveBeenCalledWith(expect.objectContaining({ identifier: 'bob_002', display_name: 'Cached Bob' })),
    );
    expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已打开 Cached Bob 的聊天');
  });

  it('renders friend profile labels without exposing the internal friend id', async () => {
    const contactsApi = createContactsApi();
    contactsApi.listFriends = vi.fn(async () => ({
      friends: [
        {
          user_id: '1001',
          friend_id: '2002',
          status: 'active',
          is_friend: true,
          created_at: '2026-04-29T12:00:00Z',
          updated_at: '2026-04-29T12:00:00Z',
          friend_profile: {
            user_id: '2002',
            identifier: 'bob_002',
            display_name: 'Bob Lin',
            name: 'Bob Lin',
            account_type: 2 as const,
          },
        },
      ],
    }));

    render(<ContactsPage userApi={createUserApi()} contactsApi={contactsApi} onStartChat={vi.fn()} />);

    const row = await screen.findByRole('button', { name: '和 bob_002 聊天' });
    expect(within(row).getByText('Bob Lin')).toBeInTheDocument();
    expect(within(row).getByText('bob_002')).toBeInTheDocument();
    expect(within(row).getByText('Agent')).toBeInTheDocument();
    expect(screen.queryByText('2002')).not.toBeInTheDocument();
  });

  it('opens a friend chat using a fallback profile when only friend id is available', async () => {
    const user = userEvent.setup();
    const contactsApi = createContactsApi();
    contactsApi.listFriends = vi.fn(async () => ({
      friends: [
        {
          user_id: '1001',
          friend_id: '2002',
          status: 'active',
          is_friend: true,
          created_at: '2026-04-29T12:00:00Z',
          updated_at: '2026-04-29T12:00:00Z',
        },
      ],
    }));
    const userApi = createUserApi();
    const onStartChat = vi.fn();

    render(<ContactsPage userApi={userApi} contactsApi={contactsApi} onStartChat={onStartChat} />);

    await user.click(await screen.findByRole('button', { name: '和 未知联系人 聊天' }));

    expect(userApi.getPublicProfileByIdentifier).not.toHaveBeenCalled();
    await waitFor(() => expect(onStartChat).toHaveBeenCalledWith(expect.objectContaining({ user_id: '2002' })));
    expect(screen.queryByText('2002')).not.toBeInTheDocument();
    expect(screen.getAllByText('未知联系人').length).toBeGreaterThan(0);
  });

  it('marks a search result as added and disabled after add-friend succeeds', async () => {
    const user = userEvent.setup();
    const contactsApi = createContactsApi();

    render(<ContactsPage userApi={createUserApi()} contactsApi={contactsApi} />);

    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));
    await user.click(await screen.findByRole('button', { name: '添加好友 bob_002' }));

    await waitFor(() => expect(screen.getAllByRole('status').map((node) => node.textContent).join(' ')).toContain('已添加好友：bob_002'));
    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    expect(within(searchRegion).getByRole('button', { name: '已添加' })).toBeDisabled();
    expect(contactsApi.addFriend).toHaveBeenCalledWith('2002');
    expect(screen.getAllByText('Bob Lin').length).toBeGreaterThan(0);
  });

  it('marks roadmap contact entries as unavailable instead of normal working actions', async () => {
    const contactsApi = createContactsApi();
    render(<ContactsPage userApi={createUserApi()} contactsApi={contactsApi} />);

    await waitFor(() => expect(contactsApi.listFriends).toHaveBeenCalled());

    expect(screen.getByLabelText('新的朋友')).not.toHaveAttribute('aria-disabled', 'true');
    for (const label of ['群聊', '标签', '公众号']) {
      const row = screen.getByLabelText(`${label} 暂未开放`);
      expect(row).toHaveAttribute('aria-disabled', 'true');
      expect(within(row).getByText('暂未开放')).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: new RegExp(label) })).not.toBeInTheDocument();
    }
  });
});
