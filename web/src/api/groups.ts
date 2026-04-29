import { type ApiClientOptions, requestEnvelope } from './shared';

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
};

export type CreateGroupRequest = {
  name: string;
  description?: string;
};

export type MemberData = {
  member: GroupMember;
  already_member: boolean;
};

export type ListMembersData = {
  group_id: string;
  members: GroupMember[];
};

export type GroupsApi = {
  getGroup: (groupId: string) => Promise<Group>;
  createGroup: (request: CreateGroupRequest) => Promise<Group>;
  joinGroup: (groupId: string, userId?: string) => Promise<MemberData>;
  leaveGroup: (groupId: string) => Promise<MemberData>;
  listMembers: (groupId: string) => Promise<ListMembersData>;
};

export function createGroupsApi(options: ApiClientOptions = {}): GroupsApi {
  return {
    getGroup(groupId: string) {
      return requestEnvelope<Group>(options, `/groups/${encodeURIComponent(groupId)}`, { method: 'GET' });
    },
    createGroup(request: CreateGroupRequest) {
      return requestEnvelope<Group>(options, '/groups', {
        method: 'POST',
        body: JSON.stringify(request),
      });
    },
    joinGroup(groupId: string, userId?: string) {
      return requestEnvelope<MemberData>(options, `/groups/${encodeURIComponent(groupId)}/members`, {
        method: 'POST',
        body: JSON.stringify(userId ? { user_id: userId } : {}),
      });
    },
    leaveGroup(groupId: string) {
      return requestEnvelope<MemberData>(options, `/groups/${encodeURIComponent(groupId)}/members/me`, {
        method: 'DELETE',
      });
    },
    listMembers(groupId: string) {
      return requestEnvelope<ListMembersData>(options, `/groups/${encodeURIComponent(groupId)}/members`, {
        method: 'GET',
      });
    },
  };
}
