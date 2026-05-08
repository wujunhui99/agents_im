import { Bot, ChevronLeft, Download, FileText, Image as ImageIcon, MessageCircle, RefreshCw, Search, SendHorizontal, UserMinus, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react';
import type { ContactsApi, Friendship } from '../../api/contacts';
import { createContactsApi } from '../../api/contacts';
import type { Group, GroupMember, GroupsApi } from '../../api/groups';
import { createGroupsApi } from '../../api/groups';
import type { MediaApi } from '../../api/media';
import { createMediaApi, uploadMediaBytes } from '../../api/media';
import type { AIHostingState, ConversationSeqState, MessageApi, ServerMessage } from '../../api/messages';
import { createMessageApi } from '../../api/messages';
import type { WebSocketFactory, WebSocketServerEvent } from '../../api/websocketClient';
import { createMessageWebSocketClient } from '../../api/websocketClient';
import type { UserApi, UserProfile } from '../../api/user';
import { createUserApi } from '../../api/user';
import { Avatar } from '../../components/ui/Avatar';
import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { friendshipToUserProfile } from '../../components/ContactsPage';
import { ListItem } from '../../components/ui/ListItem';
import { MessageBubble } from '../../components/ui/MessageBubble';
import { SearchBox } from '../../components/ui/SearchBox';
import { TextField } from '../../components/ui/TextField';
import type { ChatMessage, Conversation, MessageContentType, MessageStatus } from '../../models/messages';
import { UNKNOWN_CONTACT_LABEL, accountTypeLabel, avatarText, firstNonEmpty, profileDisplayName, profileIdentifier } from '../../utils/profileDisplay';

type MessagesPageProps = {
  currentUserId: string;
  messageApi?: MessageApi;
  mediaApi?: MediaApi;
  downloadMedia?: MediaDownloadHandler;
  contactsApi?: ContactsApi;
  groupsApi?: GroupsApi;
  userApi?: UserApi;
  webSocketUrl?: string;
  webSocketToken?: string;
  webSocketFactory?: WebSocketFactory;
  startChatSignal?: number;
  pendingChatProfile?: UserProfile | null;
  pendingGroup?: Group | null;
  onPendingChatConsumed?: () => void;
  onPendingGroupConsumed?: () => void;
};

type AttachmentKind = 'image' | 'file';
type PendingMessageInput = {
  contentType: MessageContentType;
  content: string;
};
type AIHostingPanelState = {
  state?: AIHostingState;
  loading: boolean;
  updating: boolean;
  error: string;
};
type ImageDimensions = {
  width: number;
  height: number;
};
type MediaDownloadHandler = (downloadUrl: string, filename: string) => void;
type ImageMessagePayload = {
  mediaId?: string;
  filename?: string;
  width?: number;
  height?: number;
  sizeBytes?: number;
  contentType?: string;
};
type FileMessagePayload = {
  mediaId?: string;
  filename?: string;
  sizeBytes?: number;
  contentType?: string;
};

const IMAGE_MAX_BYTES = 15 * 1024 * 1024;
const FILE_MAX_BYTES = 20 * 1024 * 1024;
const FALLBACK_CONTENT_TYPE = 'application/octet-stream';
const allowedImageMimeTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif']);

const statusLabels: Record<Exclude<MessageStatus, 'sent'>, string> = {
  sending: '发送中',
  failed: '发送失败',
};

export function MessagesPage({
  currentUserId,
  messageApi: messageApiProp,
  mediaApi: mediaApiProp,
  downloadMedia = defaultDownloadMedia,
  contactsApi: contactsApiProp,
  groupsApi: groupsApiProp,
  userApi: userApiProp,
  webSocketUrl = '/ws',
  webSocketToken,
  webSocketFactory,
  startChatSignal = 0,
  pendingChatProfile = null,
  pendingGroup = null,
  onPendingChatConsumed,
  onPendingGroupConsumed,
}: MessagesPageProps) {
  const messageApi = useMemo(() => messageApiProp ?? createMessageApi(), [messageApiProp]);
  const mediaApi = useMemo(() => mediaApiProp ?? createMediaApi(), [mediaApiProp]);
  const contactsApi = useMemo(() => contactsApiProp ?? createContactsApi(), [contactsApiProp]);
  const groupsApi = useMemo(() => groupsApiProp ?? createGroupsApi(), [groupsApiProp]);
  const userApi = useMemo(() => userApiProp ?? createUserApi(), [userApiProp]);
  const [items, setItems] = useState<Conversation[]>([]);
  const [status, setStatus] = useState('正在加载会话');
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [showStartChat, setShowStartChat] = useState(false);
  const [uploadingConversationId, setUploadingConversationId] = useState<string | null>(null);
  const [groupManagementConversationId, setGroupManagementConversationId] = useState<string | null>(null);
  const [aiHostingByConversation, setAIHostingByConversation] = useState<Record<string, AIHostingPanelState>>({});
  const readSyncsInFlight = useRef<Set<string>>(new Set());
  const selectedConversation = items.find((conversation) => conversation.id === selectedConversationId) ?? null;
  const selectedAIHosting = selectedConversation ? aiHostingByConversation[selectedConversation.id] : undefined;
  const selectedConversationSending =
    Boolean(selectedConversation && uploadingConversationId === selectedConversation.id) ||
    Boolean(selectedConversation && conversationHasInFlightSend(selectedConversation));

  useEffect(() => {
    let cancelled = false;
    async function loadConversations() {
      setStatus('正在加载会话');
      try {
        const response = await messageApi.getConversationSeqs([]);
        const states = response.states ?? response.conversations ?? response.seqs ?? [];
        const conversations = await Promise.all(states.map((state) => loadConversation(state, currentUserId, messageApi, groupsApi)));
        const needsFriendProfiles = conversations.some(
          (conversation) => conversation.chatType === 'single' && conversation.receiverId && conversation.title === UNKNOWN_CONTACT_LABEL,
        );
        const friendProfiles = needsFriendProfiles ? await loadAcceptedFriendProfileMap(contactsApi) : new Map<string, UserProfile>();
        if (!cancelled) {
          setItems((current) => mergeLoadedConversations(current, hydrateConversationTitles(conversations, friendProfiles)));
          setStatus(conversations.length > 0 ? `已加载 ${conversations.length} 个会话` : '暂无会话');
        }
      } catch (error) {
        if (!cancelled) {
          setStatus(error instanceof Error ? error.message : '加载会话失败');
        }
      }
    }

    void loadConversations();
    return () => {
      cancelled = true;
    };
  }, [currentUserId, messageApi, contactsApi, groupsApi]);

  useEffect(() => {
    if (!webSocketUrl || (!webSocketToken && !webSocketFactory) || (!webSocketFactory && !canUseNativeWebSocket())) {
      return;
    }

    const client = createMessageWebSocketClient({
      url: webSocketUrl,
      token: webSocketToken,
      webSocketFactory,
      onEvent: (event) => {
        const message = webSocketEventToServerMessage(event);
        if (!message || !conversationBelongsToCurrentUser(message, currentUserId)) {
          return;
        }
        setItems((current) => upsertLiveServerMessage(current, serverMessageToChatMessage(message, currentUserId)));
        setStatus('收到新消息');
      },
      onClose: () => {
        setStatus((current) => (current === '收到新消息' ? current : 'WebSocket 已断开'));
      },
    });

    client.connect();
    return () => client.close(1000, 'messages page unmounted');
  }, [currentUserId, webSocketUrl, webSocketToken, webSocketFactory]);

  useEffect(() => {
    if (startChatSignal > 0) {
      setShowStartChat(true);
      setSelectedConversationId(null);
    }
  }, [startChatSignal]);

  useEffect(() => {
    if (!pendingChatProfile) {
      return;
    }
    handleStartChat(pendingChatProfile);
    onPendingChatConsumed?.();
  }, [pendingChatProfile, onPendingChatConsumed]);

  useEffect(() => {
    if (!pendingGroup) {
      return;
    }
    void handleOpenGroup(pendingGroup);
    onPendingGroupConsumed?.();
  }, [pendingGroup, onPendingGroupConsumed]);

  useEffect(() => {
    if (!selectedConversationId?.startsWith('draft-single:')) {
      return;
    }

    const peerId = draftPeerId(selectedConversationId);
    const loadedConversation = items.find(
      (conversation) => conversation.id !== selectedConversationId && isSingleConversationWithPeer(conversation, peerId),
    );
    if (loadedConversation) {
      setSelectedConversationId(loadedConversation.id);
    }
  }, [items, selectedConversationId]);

  useEffect(() => {
    if (!selectedConversation) {
      return;
    }
    if (selectedConversation.unread <= 0) {
      return;
    }

    const visibleMaxSeq = maxReadableSeq(selectedConversation.messages);
    if (visibleMaxSeq === undefined || visibleMaxSeq <= (selectedConversation.hasReadSeq ?? 0)) {
      return;
    }

    const syncKey = `${selectedConversation.id}:${visibleMaxSeq}`;
    if (readSyncsInFlight.current.has(syncKey)) {
      return;
    }

    readSyncsInFlight.current.add(syncKey);
    void messageApi
      .markRead(selectedConversation.id, { hasReadSeq: visibleMaxSeq })
      .then((response) => {
        const confirmedReadSeq = response.hasReadSeq ?? visibleMaxSeq;
        setItems((current) => markConversationRead(current, selectedConversation.id, confirmedReadSeq));
      })
      .catch((error) => {
        setStatus(error instanceof Error ? error.message : '标记已读失败');
      })
      .finally(() => {
        readSyncsInFlight.current.delete(syncKey);
      });
  }, [messageApi, selectedConversation]);

  useEffect(() => {
    if (!selectedConversation || !conversationSupportsAIHosting(selectedConversation)) {
      return;
    }

    let cancelled = false;
    const conversationID = selectedConversation.id;
    setAIHostingByConversation((current) => ({
      ...current,
      [conversationID]: {
        ...(current[conversationID] ?? { updating: false }),
        loading: true,
        error: '',
      },
    }));
    void messageApi
      .getAIHosting(conversationID)
      .then((state) => {
        if (!cancelled) {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: { state, loading: false, updating: false, error: '' },
          }));
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: {
              ...(current[conversationID] ?? { updating: false }),
              loading: false,
              error: error instanceof Error ? error.message : 'AI 托管状态加载失败',
            },
          }));
        }
      });
    return () => {
      cancelled = true;
    };
  }, [messageApi, selectedConversation?.chatType, selectedConversation?.id]);

  const retryAIHosting = useCallback(
    (conversationID: string) => {
      setAIHostingByConversation((current) => ({
        ...current,
        [conversationID]: {
          ...(current[conversationID] ?? { updating: false }),
          loading: true,
          error: '',
        },
      }));
      void messageApi
        .getAIHosting(conversationID)
        .then((state) => {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: { state, loading: false, updating: false, error: '' },
          }));
        })
        .catch((error) => {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: {
              ...(current[conversationID] ?? { updating: false }),
              loading: false,
              error: error instanceof Error ? error.message : 'AI 托管状态加载失败',
            },
          }));
        });
    },
    [messageApi],
  );

  const toggleAIHosting = useCallback(
    (conversationID: string, enabled: boolean) => {
      setAIHostingByConversation((current) => ({
        ...current,
        [conversationID]: {
          ...(current[conversationID] ?? { loading: false, error: '' }),
          updating: true,
          error: '',
        },
      }));
      void messageApi
        .updateAIHosting(conversationID, { enabled })
        .then((state) => {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: { state, loading: false, updating: false, error: '' },
          }));
          setStatus(enabled ? 'AI 托管已开启' : 'AI 托管已关闭');
        })
        .catch((error) => {
          setAIHostingByConversation((current) => ({
            ...current,
            [conversationID]: {
              ...(current[conversationID] ?? { loading: false }),
              updating: false,
              error: error instanceof Error ? error.message : 'AI 托管更新失败',
            },
          }));
        });
    },
    [messageApi],
  );

  function handleStartChat(profile: UserProfile) {
    const draftConversation = userProfileToDraftConversation(profile);
    const existingConversation = items.find(
      (conversation) => conversation.chatType === 'single' && conversation.receiverId === profile.user_id,
    );
    const selectedId = existingConversation?.id ?? draftConversation.id;

    setItems((current) => upsertStartedConversation(current, existingConversation?.id, draftConversation));
    setSelectedConversationId(selectedId);
    setShowStartChat(false);
    setStatus(`已打开 ${draftConversation.title} 的聊天`);
  }

  async function handleOpenGroup(group: Group) {
    const draftConversation = groupToConversation(group);
    setItems((current) => upsertStartedConversation(current, undefined, draftConversation));
    setSelectedConversationId(draftConversation.id);
    setShowStartChat(false);
    setStatus(`已打开 ${group.name} 的聊天`);
    try {
      const members = await groupsApi.listMembers(group.group_id);
      const memberDisplayNames = groupMemberDisplayNameMap(members.members ?? []);
      setItems((current) => hydrateGroupConversationMembers(current, group, memberDisplayNames));
    } catch (error) {
      setStatus(error instanceof Error ? error.message : '加载群成员失败');
    }
  }

  function handleSend(content: string) {
    if (!selectedConversation) {
      return;
    }
    if (conversationHasInFlightSend(selectedConversation)) {
      setStatus('上一条消息发送中');
      return;
    }

    const pendingMessage = createPendingMessage(selectedConversation, { contentType: 'text', content }, currentUserId);
    setItems((current) => appendMessage(current, selectedConversation.id, pendingMessage));

    void Promise.resolve()
      .then(() => sendMessageWithApi(messageApi, pendingMessage))
      .then((sentMessage) => {
        setItems((current) => confirmSentMessage(current, selectedConversation.id, pendingMessage.id, sentMessage));
        setSelectedConversationId(sentMessage.conversationId);
      })
      .catch((error) => {
        setStatus(sendErrorMessage(error, selectedConversation.chatType));
        setItems((current) =>
          updateMessage(current, selectedConversation.id, pendingMessage.id, {
            ...pendingMessage,
            status: 'failed',
          }),
        );
      });
  }

  const handleGroupManagementUpdated = useCallback((group: Group, members: GroupMember[]) => {
    setItems((current) => hydrateGroupConversationMembers(current, group, members));
  }, []);

  async function handleSendAttachment(file: File, kind: AttachmentKind) {
    if (!selectedConversation) {
      return;
    }
    if (conversationHasInFlightSend(selectedConversation) || uploadingConversationId === selectedConversation.id) {
      setStatus('上一条消息发送中');
      return;
    }

    if (kind === 'image' && !isAllowedMessageImageType(file.type)) {
      setStatus('请选择 JPG、PNG、WebP 或 GIF 图片');
      return;
    }

    const limit = kind === 'image' ? IMAGE_MAX_BYTES : FILE_MAX_BYTES;
    if (file.size > limit) {
      setStatus(kind === 'image' ? '图片不能超过 15 MiB' : '文件不能超过 20 MiB');
      return;
    }

    const conversationAtStart = selectedConversation;
    const filename = uploadFilename(file, kind);
    const contentType = file.type || FALLBACK_CONTENT_TYPE;
    let pendingMessage: ChatMessage | null = null;

    setUploadingConversationId(conversationAtStart.id);
    setStatus(kind === 'image' ? '正在上传图片' : '正在上传文件');

    try {
      const dimensions = kind === 'image' ? await readImageDimensions(file) : undefined;
      const uploadIntent = await mediaApi.createUploadIntent({
        purpose: kind === 'image' ? 'message_image' : 'message_file',
        filename,
        contentType,
        sizeBytes: file.size,
        ...(dimensions ?? {}),
      });
      await uploadMediaBytes(uploadIntent.uploadUrl, file, contentType);
      const completed = await mediaApi.completeUpload(uploadIntent.mediaId);
      const mediaId = completed.media?.mediaId ?? uploadIntent.mediaId;
      const content =
        kind === 'image'
          ? JSON.stringify({ mediaId, filename, sizeBytes: file.size, contentType, ...(dimensions ?? {}) })
          : JSON.stringify({ mediaId, filename, sizeBytes: file.size, contentType });

      const nextPendingMessage = createPendingMessage(conversationAtStart, { contentType: kind, content }, currentUserId);
      pendingMessage = nextPendingMessage;
      setItems((current) => appendMessage(current, conversationAtStart.id, nextPendingMessage));

      const sentMessage = await sendMessageWithApi(messageApi, nextPendingMessage);
      setItems((current) => confirmSentMessage(current, conversationAtStart.id, nextPendingMessage.id, sentMessage));
      setSelectedConversationId(sentMessage.conversationId);
      setStatus(kind === 'image' ? '图片已发送' : '文件已发送');
    } catch (error) {
      setStatus(error instanceof Error ? error.message : '发送附件失败');
      if (pendingMessage) {
        const failedMessage = pendingMessage;
        setItems((current) =>
          updateMessage(current, conversationAtStart.id, failedMessage.id, {
            ...failedMessage,
            status: 'failed',
          }),
        );
      }
    } finally {
      setUploadingConversationId((current) => (current === conversationAtStart.id ? null : current));
    }
  }

  if (selectedConversation && groupManagementConversationId === selectedConversation.id) {
    return (
      <GroupManagementPanel
        currentUserId={currentUserId}
        conversation={selectedConversation}
        groupsApi={groupsApi}
        onBack={() => setGroupManagementConversationId(null)}
        onStatus={setStatus}
        onGroupUpdated={handleGroupManagementUpdated}
      />
    );
  }

  if (selectedConversation) {
    return (
      <ChatWindow
        conversation={selectedConversation}
        onBack={() => {
          setGroupManagementConversationId(null);
          setSelectedConversationId(null);
        }}
        onOpenGroupManagement={
          selectedConversation.chatType === 'group' && selectedConversation.groupId
            ? () => setGroupManagementConversationId(selectedConversation.id)
            : undefined
        }
        onSend={handleSend}
        onSendAttachment={handleSendAttachment}
        mediaApi={mediaApi}
        downloadMedia={downloadMedia}
        onStatus={setStatus}
        status={status}
        sending={selectedConversationSending}
        aiHosting={selectedAIHosting}
        onToggleAIHosting={(enabled) => toggleAIHosting(selectedConversation.id, enabled)}
        onRetryAIHosting={() => retryAIHosting(selectedConversation.id)}
      />
    );
  }

  return (
    <ConversationList
      conversations={items}
      status={status}
      userApi={userApi}
      showStartChat={showStartChat}
      onOpenStartChat={() => setShowStartChat(true)}
      onCloseStartChat={() => setShowStartChat(false)}
      onStartChat={handleStartChat}
      onSelect={(conversationId) => setSelectedConversationId(conversationId)}
    />
  );
}

function ConversationList({
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
                  {item.previewOrigin === 'ai' ? <span className="conversation-origin-badge">AI/Agent</span> : null}
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

function StartChatPanel({
  userApi,
  onStartChat,
  onClose,
}: {
  userApi: UserApi;
  onStartChat: (profile: UserProfile) => void;
  onClose: () => void;
}) {
  const [identifier, setIdentifier] = useState('');
  const [result, setResult] = useState<UserProfile | null>(null);
  const [status, setStatus] = useState('输入账号搜索用户');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = identifier.trim();

    if (!query) {
      setResult(null);
      setStatus('请输入账号');
      return;
    }

    setSubmitting(true);
    setStatus('正在搜索用户');
    try {
      const profile = await userApi.getPublicProfileByIdentifier(query);
      setResult(profile);
      setStatus(`找到 ${profileDisplayName(profile)}`);
    } catch (error) {
      setResult(null);
      setStatus(error instanceof Error ? error.message : `未找到 ${query}`);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="start-chat-card" aria-label="发起聊天">
      <div className="start-chat-heading">
        <h2>发起聊天</h2>
        <Button type="button" className="text-command" variant="text" onClick={onClose}>
          关闭
        </Button>
      </div>
      <form className="identifier-search-form" onSubmit={handleSubmit}>
        <TextField
          label="按账号搜索聊天对象"
          hideLabel
          placeholder="输入唯一账号"
          value={identifier}
          onChange={(event) => setIdentifier(event.target.value)}
          leadingIcon={<Search size={17} />}
          fieldClassName="search-box identifier-field"
        />
        <Button className="compact-command" type="submit" aria-label="搜索聊天对象" disabled={submitting}>
          <Search size={17} />
          <span>搜索</span>
        </Button>
      </form>
      <p className="inline-status" role="status">
        {status}
      </p>
      {result ? (
        <ListItem
          className="search-result"
          leading={
            <Avatar
              label={avatarText(profileDisplayName(result))}
              color="blue"
              src={result.avatar_url}
              alt={`${profileDisplayName(result)} 头像`}
            />
          }
          headline={profileDisplayName(result)}
          supportingText={
            <span className="friend-supporting-lines">
              <span>{profileIdentifier(result) ?? '资料未同步'}</span>
              <span>{accountTypeLabel(result.account_type)}</span>
            </span>
          }
          trailing={
            <Button
              className="text-command"
              variant="tonal"
              size="small"
              aria-label={`发起聊天 ${profileDisplayName(result)}`}
              onClick={() => onStartChat(result)}
            >
              发起聊天
            </Button>
          }
        />
      ) : null}
    </section>
  );
}

function ChatWindow({
  conversation,
  onBack,
  onOpenGroupManagement,
  onSend,
  onSendAttachment,
  mediaApi,
  downloadMedia,
  onStatus,
  status,
  sending,
  aiHosting,
  onToggleAIHosting,
  onRetryAIHosting,
}: {
  conversation: Conversation;
  onBack: () => void;
  onOpenGroupManagement?: () => void;
  onSend: (content: string) => void;
  onSendAttachment: (file: File, kind: AttachmentKind) => void;
  mediaApi: MediaApi;
  downloadMedia: MediaDownloadHandler;
  onStatus: (status: string) => void;
  status: string;
  sending: boolean;
  aiHosting?: AIHostingPanelState;
  onToggleAIHosting: (enabled: boolean) => void;
  onRetryAIHosting: () => void;
}) {
  const sortedMessages = useMemo(() => orderedChatMessages(conversation.messages), [conversation.messages]);
  const headerTitleContent = (
    <>
      <Avatar label={conversation.avatar} color={conversation.color} src={conversation.avatarUrl} alt={`${conversation.title} 头像`} />
      <h2>{conversation.title}</h2>
    </>
  );

  return (
    <section className="chat-window" aria-label={`${conversation.title} 聊天窗口`}>
      <header className="chat-header" role="banner" aria-label={`${conversation.title} 聊天头部`}>
        <Button variant="icon" className="chat-back-button" aria-label="返回消息列表" onClick={onBack}>
          <ChevronLeft size={24} />
        </Button>
        {onOpenGroupManagement ? (
          <button
            className="chat-header-title chat-header-title-button"
            type="button"
            aria-label={`打开群管理 ${conversation.title}`}
            onClick={onOpenGroupManagement}
          >
            {headerTitleContent}
          </button>
        ) : (
          <div className="chat-header-title">{headerTitleContent}</div>
        )}
      </header>
      {conversationSupportsAIHosting(conversation) ? (
        <AIHostingControl hosting={aiHosting} onToggle={onToggleAIHosting} onRetry={onRetryAIHosting} />
      ) : null}
      <p className="inline-status" role="status">
        {status}
      </p>
      <div className="message-thread" role="log" aria-label="聊天消息" data-testid="message-thread-scroll-region">
        {sortedMessages.map((message) => (
          <article
            className={`message-row message-${message.direction} message-origin-${message.messageOrigin}`}
            key={message.id}
            aria-label={messageAriaLabel(message)}
          >
            <div className="message-body">
              {conversation.chatType === 'group' && message.direction === 'incoming' ? (
                <span className="message-sender-name">{message.senderDisplayName ?? '群成员'}</span>
              ) : null}
              {message.messageOrigin === 'ai' ? <span className="message-origin-badge">AI/Agent</span> : null}
              {message.messageOrigin === 'system' ? <span className="message-origin-badge message-origin-system">系统</span> : null}
              {message.contentType === 'image' ? (
                <ImageMessageBubble
                  message={message}
                  mediaApi={mediaApi}
                  downloadMedia={downloadMedia}
                  onStatus={onStatus}
                  status={message.direction === 'outgoing' ? renderOutgoingMessageStatus(message, conversation.hasReadSeq) : null}
                />
              ) : message.contentType === 'file' ? (
                <FileMessageBubble
                  message={message}
                  mediaApi={mediaApi}
                  downloadMedia={downloadMedia}
                  onStatus={onStatus}
                  status={message.direction === 'outgoing' ? renderOutgoingMessageStatus(message, conversation.hasReadSeq) : null}
                />
              ) : (
                <MessageBubble
                  direction={message.direction}
                  status={message.direction === 'outgoing' ? renderOutgoingMessageStatus(message, conversation.hasReadSeq) : null}
                >
                  {renderMessageContent(message)}
                </MessageBubble>
              )}
            </div>
          </article>
        ))}
      </div>

      <SendMessageComposer onSend={onSend} onSendAttachment={onSendAttachment} sending={sending} />
    </section>
  );
}

function GroupManagementPanel({
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
        if (cancelled) {
          return;
        }
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
    return () => {
      cancelled = true;
    };
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
    if (!nextName) {
      setPanelStatus('群名称不能为空');
      return;
    }

    setSaving(true);
    setPanelStatus('正在保存群信息');
    try {
      const updated = await groupsApi.updateGroup(groupId, {
        name: nextName,
        announcement: announcementDraft.trim(),
      });
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

      <p className="inline-status" role="status">
        {panelStatus}
      </p>

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
              <article className="group-member-card" data-testid="group-member-card" key={member.user_id} aria-label={`${memberName} ${groupRoleLabel(member.role)}`}>
                <Avatar label={avatarText(memberName)} color={member.role === 'owner' ? 'orange' : member.role === 'admin' ? 'purple' : 'green'} src={member.avatar_url} alt={`${memberName} 头像`} />
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

function AIHostingControl({
  hosting,
  onToggle,
  onRetry,
}: {
  hosting?: AIHostingPanelState;
  onToggle: (enabled: boolean) => void;
  onRetry: () => void;
}) {
  const state = hosting?.state;
  const loading = hosting?.loading ?? false;
  const updating = hosting?.updating ?? false;
  const checked = Boolean(state?.enabled);
  const available = state?.available ?? true;
  const disabled = loading || updating || !available;
  const helperText =
    state?.unavailableReason ||
    hosting?.error ||
    (loading ? '正在加载 AI 托管状态' : checked ? '已开启，对方发来消息时自动代你回复' : '已关闭');

  return (
    <section className="ai-hosting-control" aria-label="AI 托管设置">
      <label className="ai-hosting-toggle">
        <span className="ai-hosting-label">
          <Bot size={17} />
          <span>AI 托管</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          aria-label="AI 托管"
          checked={checked}
          disabled={disabled}
          onChange={(event) => onToggle(event.currentTarget.checked)}
        />
      </label>
      <div className="ai-hosting-status-line">
        <span className={hosting?.error || state?.unavailableReason ? 'ai-hosting-warning' : ''}>{helperText}</span>
        {hosting?.error ? (
          <Button className="ai-hosting-retry" variant="text" size="small" type="button" onClick={onRetry} aria-label="重试 AI 托管状态">
            <RefreshCw size={14} />
            <span>重试</span>
          </Button>
        ) : null}
      </div>
    </section>
  );
}

function renderOutgoingMessageStatus(message: ChatMessage, hasReadSeq: number | undefined) {
  if (message.status === 'sent') {
    const read = message.seq !== undefined && message.seq <= (hasReadSeq ?? 0);
    return (
      <span
        className={`message-status message-status-sent message-status-check${read ? ' message-status-read' : ''}`}
        role="img"
        aria-label={read ? '对方已读' : '发送成功'}
      >
        {read ? '✔✔' : '✔'}
      </span>
    );
  }

  return <span className={`message-status message-status-${message.status}`}>{statusLabels[message.status]}</span>;
}

function FileMessageBubble({
  message,
  mediaApi,
  downloadMedia,
  onStatus,
  status,
}: {
  message: ChatMessage;
  mediaApi: MediaApi;
  downloadMedia: MediaDownloadHandler;
  onStatus: (status: string) => void;
  status: ReactNode;
}) {
  const payload = useMemo(() => parseFileMessagePayload(message.content), [message.content]);
  const mediaId = payload.mediaId;
  const filename = fileMessageFilename(payload);
  const label = fileDisplayLabel(payload);
  const metadata = fileMessageMetadata(payload);
  const [downloadError, setDownloadError] = useState('');
  const [downloading, setDownloading] = useState(false);

  async function handleDownload() {
    if (!mediaId) {
      const message = '文件信息缺失，无法下载';
      setDownloadError(message);
      onStatus(message);
      return;
    }
    setDownloading(true);
    setDownloadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      downloadMedia(result.downloadUrl, filename);
      onStatus('已获取文件下载链接');
    } catch {
      const message = '下载文件失败，请稍后重试';
      setDownloadError(message);
      onStatus(message);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className={`file-message-card file-message-card-${message.direction}`}>
      <div className="file-message-content">
        <span className="file-message-icon" aria-hidden="true">
          <FileText size={22} />
        </span>
        <span className="file-message-main">
          <span className="file-message-title">{label}</span>
          {metadata ? <span className="file-message-metadata">{metadata}</span> : null}
        </span>
        <span className="file-message-actions">
          <Button
            variant="icon"
            size="small"
            className="file-download-button"
            type="button"
            aria-label={`下载${label}`}
            onClick={handleDownload}
            disabled={downloading}
          >
            <Download size={16} />
          </Button>
          {status ? <span className="file-message-status">{status}</span> : null}
        </span>
      </div>
      {downloadError ? (
        <p className="file-message-error" role="alert">
          {downloadError}
        </p>
      ) : null}
    </div>
  );
}

function ImageMessageBubble({
  message,
  mediaApi,
  downloadMedia,
  onStatus,
  status,
}: {
  message: ChatMessage;
  mediaApi: MediaApi;
  downloadMedia: MediaDownloadHandler;
  onStatus: (status: string) => void;
  status: ReactNode;
}) {
  const payload = useMemo(() => parseImageMessagePayload(message.content), [message.content]);
  const mediaId = payload.mediaId;
  const filename = imageMessageFilename(payload);
  const label = imageDisplayLabel(payload);
  const [imageUrl, setImageUrl] = useState('');
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [downloadError, setDownloadError] = useState('');
  const [downloading, setDownloading] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!mediaId) {
      setImageUrl('');
      setLoadError('图片信息缺失，无法加载');
      return () => {
        cancelled = true;
      };
    }

    setLoading(true);
    setLoadError('');
    mediaApi
      .getDownloadURL(mediaId)
      .then((result) => {
        if (!cancelled) {
          setImageUrl(result.downloadUrl);
        }
      })
      .catch(() => {
        if (!cancelled) {
          const message = '图片加载失败，请稍后重试';
          setLoadError(message);
          onStatus(message);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [mediaApi, mediaId, onStatus]);

  async function retryLoad() {
    if (!mediaId || loading) {
      return;
    }
    setLoading(true);
    setLoadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      setImageUrl(result.downloadUrl);
    } catch {
      const message = '图片加载失败，请稍后重试';
      setLoadError(message);
      onStatus(message);
    } finally {
      setLoading(false);
    }
  }

  async function handleDownload() {
    if (!mediaId) {
      const message = '图片信息缺失，无法下载';
      setDownloadError(message);
      onStatus(message);
      return;
    }
    setDownloading(true);
    setDownloadError('');
    try {
      const result = await mediaApi.getDownloadURL(mediaId);
      downloadMedia(result.downloadUrl, filename);
      onStatus('已获取图片下载链接');
    } catch {
      const message = '下载图片失败，请稍后重试';
      setDownloadError(message);
      onStatus(message);
    } finally {
      setDownloading(false);
    }
  }

  return (
    <>
      <div className={`image-message-card image-message-card-${message.direction}`}>
        <div className="image-message-frame">
          {imageUrl ? (
            <button className="image-preview-button" type="button" aria-label={`预览${label}`} onClick={() => setPreviewOpen(true)}>
              <img src={imageUrl} alt={label} />
            </button>
          ) : (
            <button
              className="image-preview-button image-preview-placeholder"
              type="button"
              aria-label={loadError ? `重新加载${label}` : label}
              onClick={retryLoad}
              disabled={!mediaId || loading}
            >
              <ImageIcon size={22} />
              <span>{loading ? '正在加载图片' : loadError || '图片信息缺失，无法加载'}</span>
              {loadError ? <RefreshCw size={14} /> : null}
            </button>
          )}
        </div>
        <div className="image-message-actions">
          <span className="image-message-filename">{filename}</span>
          <Button
            variant="icon"
            size="small"
            className="image-download-button"
            type="button"
            aria-label={`下载${label}`}
            onClick={handleDownload}
            disabled={downloading}
          >
            <Download size={16} />
          </Button>
          {status ? <span className="image-message-status">{status}</span> : null}
        </div>
        {downloadError ? (
          <p className="image-message-error" role="alert">
            {downloadError}
          </p>
        ) : null}
      </div>
      {previewOpen ? (
        <ImagePreviewDialog
          imageUrl={imageUrl}
          label={label}
          filename={filename}
          onClose={() => setPreviewOpen(false)}
          onDownload={handleDownload}
          downloading={downloading}
          error={downloadError || loadError}
        />
      ) : null}
    </>
  );
}

function ImagePreviewDialog({
  imageUrl,
  label,
  filename,
  onClose,
  onDownload,
  downloading,
  error,
}: {
  imageUrl: string;
  label: string;
  filename: string;
  onClose: () => void;
  onDownload: () => void;
  downloading: boolean;
  error: string;
}) {
  return (
    <div className="image-preview-overlay" role="dialog" aria-modal="true" aria-label="图片预览">
      <div className="image-preview-toolbar">
        <span>{filename}</span>
        <div className="image-preview-actions">
          <Button variant="icon" type="button" aria-label={`下载${label}`} onClick={onDownload} disabled={downloading}>
            <Download size={18} />
          </Button>
          <Button variant="icon" type="button" aria-label="关闭预览" onClick={onClose}>
            <X size={20} />
          </Button>
        </div>
      </div>
      <div className="image-preview-content">
        {imageUrl ? <img src={imageUrl} alt={`预览${label}`} /> : <p>{error || '图片加载中'}</p>}
      </div>
      {error ? (
        <p className="image-preview-error" role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}

function SendMessageComposer({
  onSend,
  onSendAttachment,
  sending,
}: {
  onSend: (content: string) => void;
  onSendAttachment: (file: File, kind: AttachmentKind) => void;
  sending: boolean;
}) {
  const [draft, setDraft] = useState('');
  const trimmedDraft = draft.trim();

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !trimmedDraft) {
      return;
    }

    onSend(trimmedDraft);
    setDraft('');
  }

  function handleAttachmentChange(event: ChangeEvent<HTMLInputElement>, kind: AttachmentKind) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file || sending) {
      return;
    }
    onSendAttachment(file, kind);
  }

  return (
    <form className="message-composer" aria-label="发送消息" onSubmit={handleSubmit}>
      <label className={`message-attachment-button${sending ? ' is-disabled' : ''}`} title="发送图片">
        <ImageIcon size={18} />
        <span className="sr-only">发送图片</span>
        <input
          className="sr-only"
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif,image/*"
          aria-label="发送图片"
          disabled={sending}
          onChange={(event) => handleAttachmentChange(event, 'image')}
        />
      </label>
      <label className={`message-attachment-button${sending ? ' is-disabled' : ''}`} title="发送文件">
        <FileText size={18} />
        <span className="sr-only">发送文件</span>
        <input
          className="sr-only"
          type="file"
          aria-label="发送文件"
          disabled={sending}
          onChange={(event) => handleAttachmentChange(event, 'file')}
        />
      </label>
      <TextField
        label="输入消息"
        hideLabel
        value={draft}
        placeholder="输入消息"
        disabled={sending}
        onChange={(event) => setDraft(event.target.value)}
        fieldClassName="message-composer-field"
      />
      <Button className="message-send-button" type="submit" disabled={sending || !trimmedDraft}>
        <SendHorizontal size={17} />
        <span>{sending ? '发送中' : '发送'}</span>
      </Button>
    </form>
  );
}

async function loadConversation(
  state: ConversationSeqState,
  currentUserId: string,
  messageApi: MessageApi,
  groupsApi: GroupsApi,
): Promise<Conversation> {
  const pulled = await messageApi.pullMessages(state.conversationId, { fromSeq: 1, limit: 50, order: 'asc' });
  const conversation = conversationStateToView(state, currentUserId, pulled.messages);
  if (conversation.chatType !== 'group' || !conversation.groupId) {
    return conversation;
  }
  try {
    const [group, members] = await Promise.all([groupsApi.getGroup(conversation.groupId), groupsApi.listMembers(conversation.groupId)]);
    return applyGroupMetadata(conversation, group, members.members ?? []);
  } catch {
    return conversation;
  }
}

function userProfileToDraftConversation(profile: UserProfile): Conversation {
  const title = profileDisplayName(profile);

  return {
    id: draftConversationId(profile.user_id),
    title,
    avatar: avatarText(title),
    avatarUrl: profile.avatar_url,
    preview: '暂无消息',
    time: '',
    unread: 0,
    maxSeq: 0,
    hasReadSeq: 0,
    color: 'blue',
    chatType: 'single',
    receiverId: profile.user_id,
    messages: [],
  };
}

function groupToConversation(group: Group): Conversation {
  const title = group.name || '群聊';
  return {
    id: groupConversationId(group.group_id),
    title,
    avatar: avatarText(title),
    avatarUrl: group.avatar_url,
    preview: '暂无消息',
    time: '',
    unread: 0,
    maxSeq: 0,
    hasReadSeq: 0,
    color: 'green',
    chatType: 'group',
    groupId: group.group_id,
    messages: [],
  };
}

function mergeLoadedConversations(current: Conversation[], loaded: Conversation[]) {
  if (loaded.length === 0) {
    return current;
  }

  const mergedLoaded = loaded.map((loadedConversation) => {
    const currentConversation = current.find((conversation) => conversationsRepresentSameThread(conversation, loadedConversation));
    return currentConversation ? mergeConversation(currentConversation, loadedConversation) : loadedConversation;
  });
  const preservedCurrentConversations = current.filter(
    (conversation) =>
      shouldPreserveMissingCurrentConversation(conversation) &&
      !mergedLoaded.some((loadedConversation) => conversationsRepresentSameThread(conversation, loadedConversation)),
  );

  return [...mergedLoaded, ...preservedCurrentConversations];
}

function hydrateConversationTitles(conversations: Conversation[], friendProfiles: Map<string, UserProfile>) {
  if (friendProfiles.size === 0) {
    return conversations;
  }
  return conversations.map((conversation) => {
    if (conversation.chatType !== 'single' || !conversation.receiverId) {
      return conversation;
    }
    const profile = friendProfiles.get(conversation.receiverId);
    if (!profile) {
      return conversation;
    }
    const title = profileDisplayName(profile);
    return {
      ...conversation,
      title,
      avatar: avatarText(title),
      avatarUrl: profile.avatar_url,
    };
  });
}

function hydrateGroupConversationMembers(conversations: Conversation[], group: Group, members: GroupMember[] | Record<string, string>) {
  return conversations.map((conversation) => {
    if (conversation.chatType !== 'group' || conversation.groupId !== group.group_id) {
      return conversation;
    }
    return applyGroupMetadata(conversation, group, members);
  });
}

function applyGroupMetadata(conversation: Conversation, group: Group, members: GroupMember[] | Record<string, string>): Conversation {
  const memberDisplayNames = Array.isArray(members) ? groupMemberDisplayNameMap(members) : members;
  const title = group.name || conversation.title || '群聊';
  return {
    ...conversation,
    title,
    avatar: avatarText(title),
    avatarUrl: group.avatar_url,
    groupId: group.group_id,
    groupMemberDisplayNames: memberDisplayNames,
    messages: conversation.messages.map((message) => attachGroupSenderDisplayName(message, memberDisplayNames)),
  };
}

function groupMemberDisplayNameMap(members: GroupMember[]) {
  return members.reduce<Record<string, string>>((names, member) => {
    if (member.user_id) {
      names[member.user_id] = groupMemberDisplayName(member);
    }
    return names;
  }, {});
}

function groupMemberDisplayName(member: GroupMember) {
  return firstNonEmpty(member.display_name, member.name, member.identifier) ?? '群成员';
}

function groupAnnouncement(group: Group | null | undefined) {
  return firstNonEmpty(group?.announcement, group?.description) ?? '';
}

function currentGroupRole(group: Group | null, members: GroupMember[], currentUserId: string) {
  const memberRole = members.find((member) => member.user_id === currentUserId)?.role;
  if (memberRole) {
    return normalizeGroupRole(memberRole);
  }
  if (group?.current_user_role) {
    return normalizeGroupRole(group.current_user_role);
  }
  if (group?.creator_user_id === currentUserId) {
    return 'owner';
  }
  return 'member';
}

function normalizeGroupRole(role: string | undefined) {
  if (role === 'owner' || role === 'admin') {
    return role;
  }
  return 'member';
}

function groupRoleLabel(role: string | undefined) {
  switch (normalizeGroupRole(role)) {
    case 'owner':
      return '群主';
    case 'admin':
      return '管理员';
    default:
      return '成员';
  }
}

function canKickGroupMember(currentUserRole: string, member: GroupMember, currentUserId: string) {
  if (member.user_id === currentUserId) {
    return false;
  }
  const targetRole = normalizeGroupRole(member.role);
  if (targetRole === 'owner') {
    return false;
  }
  if (currentUserRole === 'owner') {
    return true;
  }
  return currentUserRole === 'admin' && targetRole === 'member';
}

function attachGroupSenderDisplayName(message: ChatMessage, memberDisplayNames: Record<string, string> | undefined) {
  if (message.chatType !== 'group' || message.direction !== 'incoming') {
    return message;
  }
  return {
    ...message,
    senderDisplayName: memberDisplayNames?.[message.senderId] ?? message.senderDisplayName ?? '群成员',
  };
}

async function loadAcceptedFriendProfileMap(contactsApi: ContactsApi) {
  try {
    const response = await contactsApi.listFriends();
    const friendships = response.friends ?? [];
    return friendships.reduce<Map<string, UserProfile>>((profiles, friendship) => {
      if (!isAcceptedFriendship(friendship)) {
        return profiles;
      }
      const profile = friendshipToUserProfile(friendship);
      profiles.set(profile.user_id, profile);
      return profiles;
    }, new Map<string, UserProfile>());
  } catch {
    return new Map<string, UserProfile>();
  }
}

function isAcceptedFriendship(friendship: Friendship) {
  return friendship.status === 'accepted' || friendship.status === 'active' || friendship.is_friend;
}

function upsertStartedConversation(conversations: Conversation[], existingConversationId: string | undefined, draftConversation: Conversation) {
  if (existingConversationId) {
    return conversations.map((conversation) =>
      conversation.id === existingConversationId
        ? {
            ...conversation,
            title: draftConversation.title,
            avatar: draftConversation.avatar,
            avatarUrl: draftConversation.avatarUrl,
            receiverId: draftConversation.receiverId,
          }
        : conversation,
    );
  }

  if (conversations.some((conversation) => conversation.id === draftConversation.id)) {
    return conversations.map((conversation) =>
      conversation.id === draftConversation.id ? { ...conversation, ...draftConversation, messages: conversation.messages } : conversation,
    );
  }

  return [draftConversation, ...conversations];
}

function conversationStateToView(state: ConversationSeqState, currentUserId: string, messages: ServerMessage[]): Conversation {
  const chatMessages = canonicalChatMessages(messages.map((message) => serverMessageToChatMessage(message, currentUserId)));
  const orderedMessages = orderedChatMessages(chatMessages);
  const lastMessage = orderedMessages[orderedMessages.length - 1] ?? serverLastMessage(state.lastMessage, currentUserId);
  const peerId = inferPeerId(state.conversationId, currentUserId, lastMessage);
  const isGroup = state.conversationId.startsWith('group:');
  const title = isGroup ? '群聊' : UNKNOWN_CONTACT_LABEL;
  return {
    id: state.conversationId,
    title,
    avatar: avatarText(title),
    avatarUrl: undefined,
    preview: lastMessage ? messageDisplayText(lastMessage) : '暂无消息',
    previewOrigin: lastMessage?.messageOrigin,
    time: state.maxSeqTime ? '刚刚' : '',
    unread: state.unreadCount ?? 0,
    maxSeq: state.maxSeq,
    hasReadSeq: state.hasReadSeq ?? 0,
    color: isGroup ? 'green' : 'blue',
    chatType: isGroup ? 'group' : 'single',
    receiverId: isGroup ? undefined : peerId,
    groupId: isGroup ? state.conversationId.replace(/^group:/, '') : undefined,
    messages: chatMessages,
  };
}

function appendMessage(conversations: Conversation[], conversationId: string, message: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    const nextMessages = canonicalChatMessages([...conversation.messages, message]);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, messageDisplayText(message)),
      previewOrigin: message.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, message),
      messages: nextMessages,
    };
  });
}

function canUseNativeWebSocket() {
  return typeof window !== 'undefined' && typeof window.WebSocket === 'function';
}

function updateMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId && !conversation.messages.some((message) => message.id === messageId)) {
      return conversation;
    }

    const nextMessages = upsertCanonicalMessage(conversation.messages, messageId, nextMessage);
    return {
      ...conversation,
      preview: conversationPreview(nextMessages, messageDisplayText(nextMessage)),
      time: '刚刚',
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      messages: nextMessages,
    };
  });
}

function upsertLiveServerMessage(conversations: Conversation[], message: ChatMessage) {
  let matched = false;
  const nextConversations = conversations.map((conversation) => {
    if (!conversationsRepresentSameMessageThread(conversation, message)) {
      return conversation;
    }
    matched = true;
    const nextMessage = attachGroupSenderDisplayName(message, conversation.groupMemberDisplayNames);
    const nextMessages = upsertCanonicalMessage(conversation.messages, nextMessage.id, nextMessage);
    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: conversationPreview(nextMessages, messageDisplayText(nextMessage)),
      previewOrigin: nextMessage.messageOrigin,
      time: '刚刚',
      unread: nextMessage.direction === 'incoming' ? conversation.unread + 1 : conversation.unread,
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      receiverId: liveMessagePeerTarget(nextMessage) ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: nextMessages,
    };
  });

  if (matched) {
    return nextConversations;
  }

  return [liveMessageToConversation(message), ...conversations];
}

function liveMessageToConversation(message: ChatMessage): Conversation {
  const isGroup = message.chatType === 'group';
  const title = isGroup ? '群聊' : UNKNOWN_CONTACT_LABEL;
  return {
    id: message.conversationId,
    title,
    avatar: avatarText(title),
    avatarUrl: undefined,
    preview: messageDisplayText(message),
    previewOrigin: message.messageOrigin,
    time: '刚刚',
    unread: message.direction === 'incoming' ? 1 : 0,
    maxSeq: message.seq,
    hasReadSeq: 0,
    color: isGroup ? 'green' : 'blue',
    chatType: message.chatType,
    receiverId: isGroup ? undefined : liveMessagePeerTarget(message),
    groupId: message.groupId,
    messages: [message],
  };
}

function liveMessagePeerTarget(message: ChatMessage) {
  if (message.chatType !== 'single') {
    return undefined;
  }
  return message.direction === 'incoming' ? message.senderId : message.receiverId;
}

function conversationsRepresentSameMessageThread(conversation: Conversation, message: ChatMessage) {
  if (conversation.id === message.conversationId) {
    return true;
  }
  if (conversation.chatType === 'single' && message.chatType === 'single') {
    return Boolean(conversation.receiverId && (conversation.receiverId === message.senderId || conversation.receiverId === message.receiverId));
  }
  if (conversation.chatType === 'group' && message.chatType === 'group') {
    return Boolean(conversation.groupId && conversation.groupId === message.groupId);
  }
  return false;
}

function confirmSentMessage(conversations: Conversation[], conversationId: string, messageId: string, nextMessage: ChatMessage) {
  return conversations.map((conversation) => {
    if (
      conversation.id !== conversationId &&
      conversation.id !== nextMessage.conversationId &&
      !conversation.messages.some((message) => message.id === messageId)
    ) {
      return conversation;
    }

    return {
      ...conversation,
      id: nextMessage.conversationId,
      preview: messageDisplayText(nextMessage),
      previewOrigin: nextMessage.messageOrigin,
      time: '刚刚',
      unread: 0,
      maxSeq: nextConversationMaxSeq(conversation, nextMessage),
      hasReadSeq: conversation.hasReadSeq,
      receiverId: nextMessage.receiverId ?? conversation.receiverId,
      groupId: nextMessage.groupId ?? conversation.groupId,
      messages: conversation.messages.map((message) => (message.id === messageId ? nextMessage : message)),
    };
  });
}

function markConversationRead(conversations: Conversation[], conversationId: string, hasReadSeq: number) {
  return conversations.map((conversation) => {
    if (conversation.id !== conversationId) {
      return conversation;
    }

    return applyReadSeq(conversation, hasReadSeq);
  });
}

function createPendingMessage(conversation: Conversation, messageInput: PendingMessageInput, currentUserId: string): ChatMessage {
  const now = Date.now();
  const clientMsgId = `web-${now}-${Math.random().toString(36).slice(2, 8)}`;

  return {
    id: clientMsgId,
    conversationId: conversation.id,
    clientMsgId,
    senderId: currentUserId,
    receiverId: conversation.receiverId,
    groupId: conversation.groupId,
    chatType: conversation.chatType,
    contentType: messageInput.contentType,
    content: messageInput.content,
    messageOrigin: 'human',
    sendTime: now,
    direction: 'outgoing',
    status: 'sending',
  };
}

function sendMessageWithApi(messageApi: MessageApi, message: ChatMessage): Promise<ChatMessage> {
  const request =
    message.chatType === 'group'
      ? {
          groupId: requiredField(message.groupId, 'groupId'),
          chatType: 'group' as const,
          clientMsgId: requiredField(message.clientMsgId, 'clientMsgId'),
          contentType: message.contentType,
          content: message.content,
        }
      : {
          receiverId: requiredField(message.receiverId, 'receiverId'),
          chatType: 'single' as const,
          clientMsgId: requiredField(message.clientMsgId, 'clientMsgId'),
          contentType: message.contentType,
          content: message.content,
        };

  return messageApi.sendMessage(request).then((response) => serverMessageToChatMessage(response.message, message.senderId));
}

function sendErrorMessage(error: unknown, chatType: Conversation['chatType']) {
  const message = error instanceof Error ? error.message : '';
  if (chatType === 'group' && /group member|群|forbidden|permission/i.test(message)) {
    return '没有群聊权限，无法发送消息';
  }
  return message || '发送消息失败';
}

function serverLastMessage(message: ServerMessage | undefined, currentUserId: string) {
  return message ? serverMessageToChatMessage(message, currentUserId) : undefined;
}

function webSocketEventToServerMessage(event: WebSocketServerEvent): ServerMessage | null {
  if (event.type !== 'message_received' || !isRecord(event.data)) {
    return null;
  }
  const data = event.data;
  const serverMsgId = stringField(data, 'serverMsgId', 'server_msg_id');
  const conversationId = stringField(data, 'conversationId', 'conversation_id');
  const senderId = stringField(data, 'senderId', 'sender_id');
  const chatType = stringField(data, 'chatType', 'chat_type');
  const contentType = stringField(data, 'contentType', 'content_type');
  const content = stringField(data, 'content');
  const seq = numberField(data, 'seq');
  if (!serverMsgId || !conversationId || !senderId || !chatType || !contentType || content === undefined || seq === undefined) {
    return null;
  }
  const parsedChatType = chatType === 'group' ? 'group' : 'single';
  const groupId = stringField(data, 'groupId', 'group_id') ?? (parsedChatType === 'group' ? conversationId.replace(/^group:/, '') : undefined);

  return {
    serverMsgId,
    clientMsgId: stringField(data, 'clientMsgId', 'client_msg_id') ?? '',
    conversationId,
    seq,
    senderId,
    receiverId: stringField(data, 'receiverId', 'receiver_id'),
    groupId,
    chatType: parsedChatType,
    contentType: parseMessageContentType(contentType),
    content,
    messageOrigin: messageOriginField(data),
    agentAccountId: stringField(data, 'agentAccountId', 'agent_account_id'),
    triggerServerMsgId: stringField(data, 'triggerServerMsgId', 'trigger_server_msg_id'),
    agentRunId: stringField(data, 'agentRunId', 'agent_run_id'),
    allowRecursiveTrigger: booleanField(data, 'allowRecursiveTrigger', 'allow_recursive_trigger'),
    sendTime: numberField(data, 'sendTime', 'send_time') ?? Date.now(),
    createdAt: numberField(data, 'createdAt', 'created_at') ?? Date.now(),
  };
}

function conversationBelongsToCurrentUser(message: ServerMessage, currentUserId: string) {
  if (message.chatType === 'group') {
    return true;
  }
  return message.senderId === currentUserId || message.receiverId === currentUserId;
}

function stringField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'string') {
      return field;
    }
  }
  return undefined;
}

function numberField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'number' && Number.isFinite(field)) {
      return field;
    }
  }
  return undefined;
}

function booleanField(value: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const field = value[key];
    if (typeof field === 'boolean') {
      return field;
    }
  }
  return undefined;
}

function messageOriginField(value: Record<string, unknown>) {
  const origin = stringField(value, 'messageOrigin', 'message_origin');
  return origin === 'ai' || origin === 'system' || origin === 'human' ? origin : undefined;
}

function parseMessageContentType(contentType: string): MessageContentType {
  return contentType === 'image' || contentType === 'file' ? contentType : 'text';
}

function serverMessageToChatMessage(message: ServerMessage, currentUserId: string): ChatMessage {
  return {
    id: message.serverMsgId,
    conversationId: message.conversationId,
    clientMsgId: message.clientMsgId,
    serverMsgId: message.serverMsgId,
    seq: message.seq,
    senderId: message.senderId,
    receiverId: message.receiverId,
    groupId: message.groupId,
    chatType: message.chatType,
    contentType: message.contentType,
    content: normalizeMessageContent(message.contentType, message.content),
    messageOrigin: message.messageOrigin ?? 'human',
    agentAccountId: message.agentAccountId,
    triggerServerMsgId: message.triggerServerMsgId,
    agentRunId: message.agentRunId,
    allowRecursiveTrigger: message.allowRecursiveTrigger,
    sendTime: message.sendTime,
    createdAt: message.createdAt,
    direction: message.senderId === currentUserId ? 'outgoing' : 'incoming',
    status: 'sent',
  };
}

function normalizeMessageContent(contentType: ServerMessage['contentType'], content: string) {
  if (contentType !== 'text') {
    return content;
  }

  try {
    const parsed = JSON.parse(content) as unknown;
    if (isRecord(parsed) && typeof parsed.text === 'string') {
      return parsed.text;
    }
  } catch {
    return content;
  }

  return content;
}

function renderMessageContent(message: ChatMessage) {
  if (message.contentType === 'image') {
    return (
      <span className="message-attachment message-attachment-image">
        <ImageIcon size={16} />
        <span>{messageDisplayText(message)}</span>
      </span>
    );
  }

  if (message.contentType === 'file') {
    return (
      <span className="message-attachment message-attachment-file">
        <FileText size={16} />
        <span>{messageDisplayText(message)}</span>
      </span>
    );
  }

  return message.content;
}

function messageDisplayText(message: ChatMessage) {
  if (message.contentType === 'image') {
    return imageDisplayLabel(parseImageMessagePayload(message.content));
  }

  if (message.contentType === 'file') {
    return fileDisplayLabel(parseFileMessagePayload(message.content));
  }

  return message.content;
}

function parseContentObject(content: string) {
  try {
    const parsed = JSON.parse(content) as unknown;
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function parseImageMessagePayload(content: string): ImageMessagePayload {
  const payload = parseContentObject(content);
  if (!payload) {
    return {};
  }

  return {
    mediaId: stringField(payload, 'mediaId'),
    filename: stringField(payload, 'filename'),
    width: numberField(payload, 'width'),
    height: numberField(payload, 'height'),
    sizeBytes: numberField(payload, 'sizeBytes'),
    contentType: stringField(payload, 'contentType'),
  };
}

function parseFileMessagePayload(content: string): FileMessagePayload {
  const payload = parseContentObject(content);
  if (!payload) {
    return {};
  }

  return {
    mediaId: stringField(payload, 'mediaId'),
    filename: stringField(payload, 'filename'),
    sizeBytes: numberField(payload, 'sizeBytes'),
    contentType: stringField(payload, 'contentType'),
  };
}

function imageMessageFilename(payload: ImageMessagePayload) {
  return payload.filename?.trim() || '图片消息';
}

function imageDisplayLabel(payload: ImageMessagePayload) {
  const filename = payload.filename?.trim();
  return filename ? `图片 ${filename}` : '图片消息';
}

function fileMessageFilename(payload: FileMessagePayload) {
  return payload.filename?.trim() || '文件消息';
}

function fileDisplayLabel(payload: FileMessagePayload) {
  const filename = payload.filename?.trim();
  return filename ? `文件 ${filename}` : '文件消息';
}

function fileMessageMetadata(payload: FileMessagePayload) {
  return [formatFileSize(payload.sizeBytes), payload.contentType?.trim()].filter(Boolean).join(' / ');
}

function formatFileSize(sizeBytes: number | undefined) {
  if (sizeBytes === undefined || sizeBytes < 0) {
    return undefined;
  }
  if (sizeBytes < 1024) {
    return `${sizeBytes} B`;
  }
  if (sizeBytes < 1024 * 1024) {
    return `${formatFileSizeNumber(sizeBytes / 1024)} KiB`;
  }
  return `${formatFileSizeNumber(sizeBytes / (1024 * 1024))} MiB`;
}

function formatFileSizeNumber(value: number) {
  return Number.isInteger(value) || value >= 10 ? value.toFixed(0) : value.toFixed(1);
}

function uploadFilename(file: File, kind: AttachmentKind) {
  const fallback = kind === 'image' ? 'image' : 'file';
  return file.name.trim() || fallback;
}

function isAllowedMessageImageType(contentType: string) {
  return allowedImageMimeTypes.has(contentType.toLowerCase().trim());
}

function defaultDownloadMedia(downloadUrl: string, filename: string) {
  const anchor = document.createElement('a');
  anchor.href = downloadUrl;
  anchor.download = filename;
  anchor.rel = 'noopener';
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
}

async function readImageDimensions(file: File): Promise<ImageDimensions | undefined> {
  if (typeof createImageBitmap === 'function') {
    try {
      const bitmap = await createImageBitmap(file);
      const dimensions = bitmap.width > 0 && bitmap.height > 0 ? { width: bitmap.width, height: bitmap.height } : undefined;
      bitmap.close();
      return dimensions;
    } catch {
      return undefined;
    }
  }

  if (typeof URL === 'undefined' || typeof URL.createObjectURL !== 'function' || typeof Image === 'undefined') {
    return undefined;
  }

  return new Promise((resolve) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();
    const timeout = window.setTimeout(() => {
      cleanup();
      resolve(undefined);
    }, 250);
    function cleanup() {
      window.clearTimeout(timeout);
      URL.revokeObjectURL(objectUrl);
      image.onload = null;
      image.onerror = null;
    }
    image.onload = () => {
      cleanup();
      const width = image.naturalWidth || image.width;
      const height = image.naturalHeight || image.height;
      resolve(width > 0 && height > 0 ? { width, height } : undefined);
    };
    image.onerror = () => {
      cleanup();
      resolve(undefined);
    };
    image.src = objectUrl;
  });
}

function inferPeerId(conversationId: string, currentUserId: string, lastMessage?: ChatMessage) {
  if (lastMessage) {
    if (lastMessage.senderId && lastMessage.senderId !== currentUserId) {
      return lastMessage.senderId;
    }
    if (lastMessage.receiverId && lastMessage.receiverId !== currentUserId) {
      return lastMessage.receiverId;
    }
  }
  if (conversationId.startsWith('single:')) {
    return conversationId
      .replace(/^single:/, '')
      .split(':')
      .find((part) => part && part !== currentUserId);
  }
  return undefined;
}

function requiredField(value: string | undefined, fieldName: string) {
  if (!value) {
    throw new Error(`${fieldName} is required`);
  }
  return value;
}

function draftConversationId(userId: string) {
  return `draft-single:${userId}`;
}

function groupConversationId(groupId: string) {
  return `group:${groupId}`;
}

function draftPeerId(conversationId: string) {
  return conversationId.replace(/^draft-single:/, '');
}

function conversationHasInFlightSend(conversation: Conversation) {
  return conversation.messages.some((message) => message.status === 'sending' && !hasAuthoritativeSeq(message));
}

function conversationSupportsAIHosting(conversation: Conversation) {
  return conversation.chatType === 'single' && !conversation.id.startsWith('draft-single:') && conversation.id.startsWith('single:');
}

function conversationPreview(messages: ChatMessage[], fallback: string) {
  const orderedMessages = orderedChatMessages(messages);
  const lastMessage = orderedMessages[orderedMessages.length - 1];
  return lastMessage ? messageDisplayText(lastMessage) : fallback;
}

function nextConversationMaxSeq(conversation: Conversation, message: ChatMessage) {
  const nextSeq = authoritativeSeq(message) ?? 0;
  return Math.max(conversation.maxSeq ?? 0, nextSeq);
}

function maxReadableSeq(messages: ChatMessage[]) {
  return orderedChatMessages(messages).reduce<number | undefined>((maxSeq, message) => {
    const seq = authoritativeSeq(message);
    if (seq === undefined) {
      return maxSeq;
    }
    return maxSeq === undefined ? seq : Math.max(maxSeq, seq);
  }, undefined);
}

function orderedChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages)
    .sort((left, right) => compareMessageEntries(left, right))
    .map((entry) => entry.message);
}

function canonicalChatMessages(messages: ChatMessage[]) {
  return canonicalMessageEntries(messages).map((entry) => entry.message);
}

function upsertCanonicalMessage(messages: ChatMessage[], pendingMessageId: string, nextMessage: ChatMessage) {
  let replaced = false;
  const nextIdentityKeys = new Set(messageIdentityKeys(nextMessage));
  const updatedMessages = messages.map((message) => {
    if (message.id === pendingMessageId || messageIdentityKeys(message).some((key) => nextIdentityKeys.has(key))) {
      replaced = true;
      return chooseCanonicalMessage(message, nextMessage);
    }
    return message;
  });

  if (!replaced) {
    updatedMessages.push(nextMessage);
  }
  return canonicalChatMessages(updatedMessages);
}

type MessageEntry = {
  message: ChatMessage;
  index: number;
};

function canonicalMessageEntries(messages: ChatMessage[]) {
  const entries = new Map<string, MessageEntry>();
  const aliases = new Map<string, string>();

  messages.forEach((message, index) => {
    const identityKeys = messageIdentityKeys(message);
    const existingKey =
      identityKeys.map((key) => aliases.get(key)).find((key): key is string => Boolean(key)) ??
      identityKeys.find((key) => entries.has(key)) ??
      identityKeys[0];
    const existing = entries.get(existingKey);
    if (!existing) {
      entries.set(existingKey, { message, index });
    } else {
      entries.set(existingKey, {
        message: chooseCanonicalMessage(existing.message, message),
        index: Math.min(existing.index, index),
      });
    }

    identityKeys.forEach((key) => aliases.set(key, existingKey));
  });

  return Array.from(entries.values());
}

function compareMessageEntries(left: MessageEntry, right: MessageEntry) {
  const leftSeq = authoritativeSeq(left.message);
  const rightSeq = authoritativeSeq(right.message);
  if (leftSeq !== undefined && rightSeq !== undefined && leftSeq !== rightSeq) {
    return leftSeq - rightSeq;
  }
  if (leftSeq !== undefined && rightSeq === undefined) {
    return -1;
  }
  if (leftSeq === undefined && rightSeq !== undefined) {
    return 1;
  }
  if (leftSeq === undefined && rightSeq === undefined) {
    return left.index - right.index;
  }
  return stableMessageTieBreaker(left.message).localeCompare(stableMessageTieBreaker(right.message));
}

function hasAuthoritativeSeq(message: ChatMessage): message is ChatMessage & { seq: number } {
  return authoritativeSeq(message) !== undefined;
}

function authoritativeSeq(message: ChatMessage) {
  return typeof message.seq === 'number' && Number.isFinite(message.seq) && message.seq > 0 ? message.seq : undefined;
}

function messageIdentityKeys(message: ChatMessage) {
  return uniqueStrings([
    message.serverMsgId ? `server:${message.serverMsgId}` : '',
    message.clientMsgId ? `client:${message.clientMsgId}` : '',
    `local:${message.id}`,
  ]);
}

function chooseCanonicalMessage(current: ChatMessage, candidate: ChatMessage) {
  const currentCanonical = isServerCanonical(current);
  const candidateCanonical = isServerCanonical(candidate);
  if (candidateCanonical && !currentCanonical) {
    return candidate;
  }
  if (currentCanonical && !candidateCanonical) {
    return current;
  }
  if (candidate.status === 'sent' && current.status !== 'sent') {
    return candidate;
  }
  return candidate;
}

function isServerCanonical(message: ChatMessage) {
  return Boolean(message.serverMsgId) || hasAuthoritativeSeq(message);
}

function stableMessageTieBreaker(message: ChatMessage) {
  return message.serverMsgId ?? message.clientMsgId ?? message.id;
}

function uniqueStrings(values: string[]) {
  return values.filter((value, index) => value !== '' && values.indexOf(value) === index);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function messageAriaLabel(message: ChatMessage) {
  const label = messageDisplayText(message);
  if (message.messageOrigin === 'ai') {
    return `AI Agent 消息：${label}`;
  }
  if (message.messageOrigin === 'system') {
    return `系统消息：${label}`;
  }
  return message.direction === 'outgoing' ? `我发送的消息：${label}` : `收到的消息：${label}`;
}

function conversationsRepresentSameThread(left: Conversation, right: Conversation) {
  if (left.id === right.id) {
    return true;
  }
  if (left.chatType === 'single' && right.chatType === 'single') {
    return Boolean(left.receiverId && left.receiverId === right.receiverId);
  }
  if (left.chatType === 'group' && right.chatType === 'group') {
    return Boolean(left.groupId && left.groupId === right.groupId);
  }
  return false;
}

function isLocalConversation(conversation: Conversation) {
  return (
    conversation.id.startsWith('draft-') ||
    conversation.messages.some((message) => message.status !== 'sent' || !isServerCanonical(message))
  );
}

function shouldPreserveMissingCurrentConversation(conversation: Conversation) {
  return isLocalConversation(conversation) || conversation.messages.length > 0;
}

function mergeConversation(current: Conversation, loaded: Conversation): Conversation {
  const groupMemberDisplayNames =
    current.chatType === 'group' || loaded.chatType === 'group'
      ? { ...(loaded.groupMemberDisplayNames ?? {}), ...(current.groupMemberDisplayNames ?? {}) }
      : undefined;
  const messages = canonicalChatMessages([...current.messages, ...loaded.messages]).map((message) =>
    attachGroupSenderDisplayName(message, groupMemberDisplayNames),
  );
  const orderedMessages = orderedChatMessages(messages);
  const lastMessage = orderedMessages[orderedMessages.length - 1];
  const title = shouldKeepCurrentTitle(current, loaded) ? current.title : loaded.title;
  const maxSeq = Math.max(current.maxSeq ?? 0, loaded.maxSeq ?? 0, maxReadableSeq(messages) ?? 0);
  const hasReadSeq = Math.max(current.hasReadSeq ?? 0, loaded.hasReadSeq ?? 0);
  const unread = unreadAfterRead({ ...loaded, maxSeq, messages }, hasReadSeq);

  return {
    ...loaded,
    title,
    avatar: title === current.title ? current.avatar : loaded.avatar,
    preview: lastMessage ? messageDisplayText(lastMessage) : loaded.preview,
    avatarUrl: title === current.title ? current.avatarUrl : loaded.avatarUrl,
    previewOrigin: lastMessage?.messageOrigin ?? loaded.previewOrigin,
    time: lastMessage ? '刚刚' : loaded.time,
    unread,
    maxSeq,
    hasReadSeq,
    groupMemberDisplayNames,
    messages,
  };
}

function applyReadSeq(conversation: Conversation, hasReadSeq: number): Conversation {
  const nextHasReadSeq = Math.max(conversation.hasReadSeq ?? 0, hasReadSeq);
  return {
    ...conversation,
    hasReadSeq: nextHasReadSeq,
    unread: unreadAfterRead(conversation, nextHasReadSeq),
  };
}

function unreadAfterRead(conversation: Conversation, hasReadSeq: number) {
  const maxSeq = conversation.maxSeq ?? maxReadableSeq(conversation.messages) ?? 0;
  if (hasReadSeq >= maxSeq) {
    return 0;
  }

  const previousReadSeq = conversation.hasReadSeq ?? 0;
  const readDelta = Math.max(0, hasReadSeq - previousReadSeq);
  return Math.max(0, conversation.unread - readDelta);
}

function shouldKeepCurrentTitle(current: Conversation, loaded: Conversation) {
  if (!current.id.startsWith('draft-')) {
    return false;
  }
  return Boolean(current.title && current.title !== loaded.title && current.title !== current.receiverId);
}

function isSingleConversationWithPeer(conversation: Conversation, peerId: string) {
  return conversation.chatType === 'single' && conversation.receiverId === peerId;
}
