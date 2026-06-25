import type { Group, GroupMember } from '../../../api/groups';
import type { ChatMessage } from '../../../models/messages';
import { firstNonEmpty } from '../../../utils/profileDisplay';

export function groupMemberDisplayNameMap(members: GroupMember[]) {
  return members.reduce<Record<string, string>>((names, member) => {
    if (member.user_id) {
      names[member.user_id] = groupMemberDisplayName(member);
    }
    return names;
  }, {});
}

export function groupMemberDisplayName(member: GroupMember) {
  return firstNonEmpty(member.display_name, member.name, member.identifier) ?? '群成员';
}

export function groupAnnouncement(group: Group | null | undefined) {
  return firstNonEmpty(group?.announcement, group?.description) ?? '';
}

export function currentGroupRole(group: Group | null, members: GroupMember[], currentUserId: string) {
  const memberRole = members.find((member) => member.user_id === currentUserId)?.role;
  if (memberRole) return normalizeGroupRole(memberRole);
  if (group?.current_user_role) return normalizeGroupRole(group.current_user_role);
  if (group?.creator_user_id === currentUserId) return 'owner';
  return 'member';
}

export function normalizeGroupRole(role: string | undefined) {
  if (role === 'owner' || role === 'admin') return role;
  return 'member';
}

export function groupRoleLabel(role: string | undefined) {
  switch (normalizeGroupRole(role)) {
    case 'owner': return '群主';
    case 'admin': return '管理员';
    default: return '成员';
  }
}

export function canKickGroupMember(currentUserRole: string, member: GroupMember, currentUserId: string) {
  if (member.user_id === currentUserId) return false;
  const targetRole = normalizeGroupRole(member.role);
  if (targetRole === 'owner') return false;
  if (currentUserRole === 'owner') return true;
  return currentUserRole === 'admin' && targetRole === 'member';
}

export function attachGroupSenderDisplayName(message: ChatMessage, memberDisplayNames: Record<string, string> | undefined) {
  if (message.chatType !== 'group' || message.direction !== 'incoming') return message;
  return {
    ...message,
    senderDisplayName: memberDisplayNames?.[message.senderId] ?? message.senderDisplayName ?? '群成员',
  };
}
