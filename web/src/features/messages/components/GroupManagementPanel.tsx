import { ChevronLeft, UserMinus } from 'lucide-react';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import type { Group, GroupMember, GroupsApi } from '../../../api/groups';
import { Avatar } from '../../../components/ui/Avatar';
import { Button } from '../../../components/ui/Button';
import { TextField } from '../../../components/ui/TextField';
import type { Conversation } from '../../../models/messages';
import { avatarText } from '../../../utils/profileDisplay';
import { canKickGroupMember, currentGroupRole, groupAnnouncement, groupMemberDisplayName, groupRoleLabel } from '../utils/groupUtils';
import { requiredField } from '../utils/conversationUtils';

export function GroupManagementPanel({
  currentUserId,
  conversation,
  groupsApi,
  onBack,
  onStatus,
  onGroupUpdated,
}: {
  currentUserId: string;
  conversation: Conversation;
  groupsApi: GroupsApi;
  onBack: () => void;
  onStatus: (status: string) => void;
  onGroupUpdated: (group: Group, members: GroupMember[]) => void;
}) {
  const groupId = requiredField(conversation.groupId, 'groupId');
  const fallbackTitleRef = useRef(conversation.title);
  const [group, setGroup] = useState<Group | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [nameDraft, setNameDraft] = useState(conversation.title);
  const [announcementDraft, setAnnouncementDraft] = useState('');
  const [panelStatus, setPanelStatus] = useState('正在加载群管理');
  const [saving, setSaving] = useState(false);
  const [kickingUserId, setKickingUserId] = useState<string | null>(null);
  const currentUserRole = currentGroupRole(group, members, currentUserId);
  const canManage = currentUserRole === 'owner' || currentUserRole === 'admin';
  const displayGroupName = group?.name || conversation.title;
  const announcement = groupAnnouncement(group);

  useEffect(() => {
    let cancelled = false;
    async function loadDetails() {
      setPanelStatus('正在加载群管理');
      try {
        const [nextGroup, nextMembers] = await Promise.all([groupsApi.getGroup(groupId), groupsApi.listMembers(groupId)]);
        if (cancelled) return;
        const activeMembers = nextMembers.members ?? [];
        setGroup(nextGroup);
        setMembers(activeMembers);
        setNameDraft(nextGroup.name || fallbackTitleRef.current);
        setAnnouncementDraft(groupAnnouncement(nextGroup));
        setPanelStatus('群管理已加载');
        onGroupUpdated(nextGroup, activeMembers);
      } catch (error) {
        if (!cancelled) {
          const message = error instanceof Error ? error.message : '加载群管理失败';
          setPanelStatus(message);
          onStatus(message);
        }
      }
    }
    void loadDetails();
    return () => { cancelled = true; };
  }, [groupId, groupsApi, onGroupUpdated, onStatus]);

  async function reloadAfterKick() {
    const [nextGroup, nextMembers] = await Promise.all([groupsApi.getGroup(groupId), groupsApi.listMembers(groupId)]);
    const activeMembers = nextMembers.members ?? [];
    setGroup(nextGroup);
    setMembers(activeMembers);
    setNameDraft(nextGroup.name || fallbackTitleRef.current);
    setAnnouncementDraft(groupAnnouncement(nextGroup));
    onGroupUpdated(nextGroup, activeMembers);
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextName = nameDraft.trim();
    if (!nextName) { setPanelStatus('群名称不能为空'); return; }
    setSaving(true);
    setPanelStatus('正在保存群信息');
    try {
      const updated = await groupsApi.updateGroup(groupId, { name: nextName, announcement: announcementDraft.trim() });
      setGroup(updated);
      setNameDraft(updated.name || nextName);
      setAnnouncementDraft(groupAnnouncement(updated));
      onGroupUpdated(updated, members);
      setPanelStatus('群信息已更新');
      onStatus('群信息已更新');
    } catch (error) {
      const message = error instanceof Error ? error.message : '更新群信息失败';
      setPanelStatus(message);
      onStatus(message);
    } finally {
      setSaving(false);
    }
  }

  async function handleKick(member: GroupMember) {
    setKickingUserId(member.user_id);
    setPanelStatus(`正在移除 ${groupMemberDisplayName(member)}`);
    try {
      await groupsApi.kickMember(groupId, member.user_id);
      await reloadAfterKick();
      setPanelStatus(`已移除 ${groupMemberDisplayName(member)}`);
      onStatus(`已移除 ${groupMemberDisplayName(member)}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : '移除群成员失败';
      setPanelStatus(message);
      onStatus(message);
    } finally {
      setKickingUserId(null);
    }
  }

  return (
    <section className="group-management" aria-label={`${displayGroupName} 群管理`}>
      <header className="chat-header group-management-header" role="banner" aria-label={`${displayGroupName} 群管理头部`}>
        <Button variant="icon" className="chat-back-button" aria-label="返回聊天" onClick={onBack}>
          <ChevronLeft size={24} />
        </Button>
        <div className="chat-header-title">
          <Avatar label={avatarText(displayGroupName)} color="green" src={group?.avatar_url || conversation.avatarUrl} alt={`${displayGroupName} 头像`} />
          <h2>群管理</h2>
        </div>
      </header>

      <p className="inline-status" role="status">{panelStatus}</p>

      <section className="group-management-summary" aria-label="群资料">
        <Avatar label={avatarText(displayGroupName)} color="green" size="large" src={group?.avatar_url || conversation.avatarUrl} alt={`${displayGroupName} 头像`} />
        <div className="group-management-summary-text">
          <h3>{displayGroupName}</h3>
          <span>{groupRoleLabel(currentUserRole)}</span>
        </div>
      </section>

      {canManage ? (
        <form className="group-management-form" aria-label="编辑群资料" onSubmit={handleSave}>
          <TextField label="群名称" value={nameDraft} onChange={(event) => setNameDraft(event.currentTarget.value)} />
          <TextField label="群公告" value={announcementDraft} onChange={(event) => setAnnouncementDraft(event.currentTarget.value)} />
          <Button className="compact-command" type="submit" disabled={saving}>
            <span>{saving ? '保存中' : '保存群信息'}</span>
          </Button>
        </form>
      ) : (
        <section className="group-management-readonly" aria-label="群资料只读">
          <div>
            <span>群名称</span>
            <p>{displayGroupName}</p>
          </div>
          <div>
            <span>群公告</span>
            <p>{announcement || '暂无公告'}</p>
          </div>
        </section>
      )}

      <section className="group-management-members" aria-label="群成员">
        <div className="panel-heading">
          <h2>群成员</h2>
          <span>{members.length} 人</span>
        </div>
        <div className="group-member-grid" data-testid="group-member-grid">
          {members.map((member) => {
            const memberName = groupMemberDisplayName(member);
            const kickable = canKickGroupMember(currentUserRole, member, currentUserId);
            return (
              <article
                className="group-member-card"
                data-testid="group-member-card"
                key={member.user_id}
                aria-label={`${memberName} ${groupRoleLabel(member.role)}`}
              >
                <Avatar
                  label={avatarText(memberName)}
                  color={member.role === 'owner' ? 'orange' : member.role === 'admin' ? 'purple' : 'green'}
                  src={member.avatar_url}
                  alt={`${memberName} 头像`}
                />
                <span className="group-member-name">{memberName}</span>
                <span className="group-member-role">{groupRoleLabel(member.role)}</span>
                {kickable ? (
                  <Button
                    className="group-member-kick-button"
                    variant="icon"
                    size="small"
                    type="button"
                    aria-label={`踢出 ${memberName}`}
                    disabled={kickingUserId === member.user_id}
                    onClick={() => handleKick(member)}
                  >
                    <UserMinus size={15} />
                  </Button>
                ) : null}
              </article>
            );
          })}
        </div>
      </section>
    </section>
  );
}
