import { type ApiClientOptions, requestEnvelope } from './shared';

export type Friendship = {
  user_id: string;
  friend_id: string;
  status: string;
  is_friend: boolean;
  created_at: string;
  updated_at: string;
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

export function createContactsApi(options: ApiClientOptions = {}): ContactsApi {
  return {
    listFriends() {
      return requestEnvelope<ListFriendsData>(options, '/friends', { method: 'GET' });
    },
    addFriend(userId: string) {
      return requestEnvelope<AddFriendData>(options, '/friends', {
        method: 'POST',
        body: JSON.stringify({ user_id: userId }),
      });
    },
    deleteFriend(userId: string) {
      return requestEnvelope<DeleteFriendData>(options, `/friends/${encodeURIComponent(userId)}`, {
        method: 'DELETE',
      });
    },
  };
}
