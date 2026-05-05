import { createApiClient, type ApiClient } from './client';

export type Group = {
  group_id: string;
  name: string;
  description: string;
  creator_user_id: string;
  created_at: string;
  updated_at: string;
};

export type GroupMember = {
  group_id: string;
  user_id: string;
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
  joinGroup: (groupId: string, userId?: string) => Promise<MemberData>;
  leaveGroup: (groupId: string) => Promise<MemberData>;
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
    joinGroup(groupId: string, userId?: string) {
      return api.post<MemberData>(`/groups/${encodeURIComponent(groupId)}/members`, userId ? { user_id: userId } : {});
    },
    leaveGroup(groupId: string) {
      return api.delete<MemberData>(`/groups/${encodeURIComponent(groupId)}/members/me`);
    },
    listMembers(groupId: string) {
      return api.get<ListMembersData>(`/groups/${encodeURIComponent(groupId)}/members`);
    },
  };
}
