import { createApiClient, type ApiClient } from './client';

export type Group = {
  group_id: string;
  name: string;
  description: string;
  announcement?: string;
  avatar_media_id?: string;
  avatar_url?: string;
  creator_user_id: string;
  current_user_role?: GroupRole;
  created_at: string;
  updated_at: string;
};

export type GroupRole = 'owner' | 'admin' | 'member' | string;

export type GroupMember = {
  group_id: string;
  user_id: string;
  role?: GroupRole;
  state: string;
  joined_at: string;
  left_at: string;
  identifier?: string;
  display_name?: string;
  name?: string;
  avatar_media_id?: string;
  avatar_url?: string;
  avatar_url_expires_at?: number;
};

export type CreateGroupRequest = {
  name: string;
  description?: string;
  member_user_ids?: string[];
};

export type UpdateGroupRequest = {
  name?: string;
  description?: string;
  announcement?: string;
};

export type MemberData = {
  member: GroupMember;
  already_member: boolean;
};

export type ListMembersData = {
  group_id: string;
  members: GroupMember[];
};

export type ListGroupsData = {
  groups: Group[];
};

export type GroupsApi = {
  listGroups: () => Promise<ListGroupsData>;
  getGroup: (groupId: string) => Promise<Group>;
  createGroup: (request: CreateGroupRequest) => Promise<Group>;
  updateGroup: (groupId: string, request: UpdateGroupRequest) => Promise<Group>;
  joinGroup: (groupId: string, userId?: string) => Promise<MemberData>;
  leaveGroup: (groupId: string) => Promise<MemberData>;
  kickMember: (groupId: string, userId: string) => Promise<MemberData>;
  listMembers: (groupId: string) => Promise<ListMembersData>;
};

export function createGroupsApi(api: ApiClient = createApiClient()): GroupsApi {
  return {
    listGroups() {
      return api.get<ListGroupsData>('/groups');
    },
    getGroup(groupId: string) {
      return api.get<Group>(`/groups/${encodeURIComponent(groupId)}`);
    },
    createGroup(request: CreateGroupRequest) {
      return api.post<Group>('/groups', request);
    },
    updateGroup(groupId: string, request: UpdateGroupRequest) {
      return api.patch<Group>(`/groups/${encodeURIComponent(groupId)}`, request);
    },
    joinGroup(groupId: string, userId?: string) {
      return api.post<MemberData>(`/groups/${encodeURIComponent(groupId)}/members`, userId ? { user_id: userId } : {});
    },
    leaveGroup(groupId: string) {
      return api.delete<MemberData>(`/groups/${encodeURIComponent(groupId)}/members/me`);
    },
    kickMember(groupId: string, userId: string) {
      return api.delete<MemberData>(`/groups/${encodeURIComponent(groupId)}/members/${encodeURIComponent(userId)}`);
    },
    listMembers(groupId: string) {
      return api.get<ListMembersData>(`/groups/${encodeURIComponent(groupId)}/members`);
    },
  };
}
