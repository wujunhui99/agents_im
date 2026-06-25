import { MessageCircle } from 'lucide-react';
import type { UserApi, UserProfile } from '../../../api/user';
import { Avatar } from '../../../components/ui/Avatar';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { ListItem } from '../../../components/ui/ListItem';
import { SearchBox } from '../../../components/ui/SearchBox';
import type { Conversation } from '../../../models/messages';
import { StartChatPanel } from './StartChatPanel';

export function ConversationList({
  conversations,
  status,
  userApi,
  showStartChat,
  onOpenStartChat,
  onCloseStartChat,
  onStartChat,
  onSelect,
}: {
  conversations: Conversation[];
  status: string;
  userApi: UserApi;
  showStartChat: boolean;
  onOpenStartChat: () => void;
  onCloseStartChat: () => void;
  onStartChat: (profile: UserProfile) => void;
  onSelect: (conversationId: string) => void;
}) {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
      {showStartChat ? <StartChatPanel userApi={userApi} onStartChat={onStartChat} onClose={onCloseStartChat} /> : null}
      <p className="inline-status" role="status">
        {status}
      </p>
      {conversations.length === 0 ? (
        <div className="empty-state empty-state-action">
          <p>暂无会话</p>
          <Button className="compact-command" type="button" onClick={onOpenStartChat}>
            <MessageCircle size={17} />
            <span>发起聊天</span>
          </Button>
        </div>
      ) : null}
      <Card className="list-card conversation-list" role="list" aria-label="消息列表">
        {conversations.map((item) => (
          <div className="conversation-list-item" role="listitem" key={item.id}>
            <ListItem
              className="conversation-row conversation-button"
              onClick={() => onSelect(item.id)}
              leading={<Avatar label={item.avatar} color={item.color} src={item.avatarUrl} alt={`${item.title} 头像`} />}
              headline={
                <span className="row-title-line">
                  <span>{item.title}</span>
                  <time>{item.time}</time>
                </span>
              }
              supportingText={
                <>
                  {item.previewOrigin === 'ai' ? <span className="conversation-origin-badge">AI Agent</span> : null}
                  {item.previewOrigin === 'system' ? <span className="conversation-origin-badge conversation-origin-system">系统</span> : null}
                  {item.preview}
                </>
              }
              trailing={
                item.unread > 0 ? (
                  <Badge tone="error" className="unread-badge">
                    {item.unread}
                  </Badge>
                ) : null
              }
            />
          </div>
        ))}
      </Card>
    </div>
  );
}
