import { createApiClient, type ApiClient } from './client';
import type { UserProfile } from './user';

export type Friendship = {
  user_id: string;
  friend_id: string;
  status: string;
  is_friend: boolean;
  created_at: string;
  updated_at: string;
  friend?: Partial<UserProfile>;
  friend_profile?: Partial<UserProfile>;
  profile?: Partial<UserProfile>;
  identifier?: string;
  display_name?: string;
  name?: string;
  friend_identifier?: string;
  friend_display_name?: string;
  friend_name?: string;
};

export type ListFriendsData = {
  friends: Friendship[];
};

export type AddFriendData = {
  friendship: Friendship;
  created: boolean;
};

export type DeleteFriendData = {
  friendship: Friendship;
  deleted: boolean;
};

export type ContactsApi = {
  listFriends: () => Promise<ListFriendsData>;
  addFriend: (userId: string) => Promise<AddFriendData>;
  deleteFriend: (userId: string) => Promise<DeleteFriendData>;
};

export function createContactsApi(api: ApiClient = createApiClient()): ContactsApi {
  return {
    listFriends() {
      return api.get<ListFriendsData>('/friends');
    },
    addFriend(userId: string) {
      return api.post<AddFriendData>('/friends', { user_id: userId });
    },
    deleteFriend(userId: string) {
      return api.delete<DeleteFriendData>(`/friends/${encodeURIComponent(userId)}`);
    },
  };
}
