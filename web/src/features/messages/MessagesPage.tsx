import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createContactsApi } from '../../api/contacts';
import type { Group, GroupMember } from '../../api/groups';
import { createGroupsApi } from '../../api/groups';
import { createMediaApi } from '../../api/media';
import { createMessageApi } from '../../api/messages';
import { createUserApi } from '../../api/user';
import { createMessageWebSocketClient } from '../../api/websocketClient';
import { uploadFileToMedia } from '../../utils/mediaTransfer';
import { UNKNOWN_CONTACT_LABEL } from '../../utils/profileDisplay';
import type { AIHostingPanelState, AttachmentKind, MessagesPageProps } from './types';
import { FALLBACK_CONTENT_TYPE, FILE_MAX_BYTES, IMAGE_MAX_BYTES } from './types';
import { ChatWindow } from './components/ChatWindow';
import { ConversationList } from './components/ConversationList';
import { GroupManagementPanel } from './components/GroupManagementPanel';
import {
  appendMessage,
  confirmSentMessage,
  createPendingMessage,
  markConversationRead,
  sendErrorMessage,
  sendMessageWithApi,
  updateMessage,
  upsertLiveServerMessage,
  upsertStartedConversation,
} from './utils/conversationReducers';
import {
  canUseNativeWebSocket,
  conversationHasInFlightSend,
  conversationSupportsAIHosting,
  conversationStateToView,
  draftPeerId,
  groupToConversation,
  hydrateConversationTitles,
  hydrateGroupConversationMembers,
  isSingleConversationWithPeer,
  loadAcceptedFriendProfileMap,
  loadConversation,
  mergeLoadedConversations,
  userProfileToDraftConversation,
} from './utils/conversationUtils';
import type { UserProfile } from '../../api/user';
import { groupMemberDisplayNameMap } from './utils/groupUtils';
import { defaultDownloadMedia, isAllowedMessageImageType, readImageDimensions, uploadFilename } from './utils/mediaUtils';
import { maxReadableSeq } from './utils/messageOrdering';
import { conversationBelongsToCurrentUser, serverMessageToChatMessage, webSocketEventToServerMessage } from './utils/serverMessageParser';

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
  onAuthFailure,
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

  const [items, setItems] = useState<ReturnType<typeof mergeLoadedConversations>>([]);
  const [status, setStatus] = useState('正在加载会话');
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [showStartChat, setShowStartChat] = useState(false);
  const [uploadingConversationId, setUploadingConversationId] = useState<string | null>(null);
  const [groupManagementConversationId, setGroupManagementConversationId] = useState<string | null>(null);
  const [aiHostingByConversation, setAIHostingByConversation] = useState<Record<string, AIHostingPanelState>>({});
  const readSyncsInFlight = useRef<Set<string>>(new Set());

  const selectedConversation = items.find((c) => c.id === selectedConversationId) ?? null;
  const selectedAIHosting = selectedConversation ? aiHostingByConversation[selectedConversation.id] : undefined;
  const selectedConversationSending =
    Boolean(selectedConversation && uploadingConversationId === selectedConversation.id) ||
    Boolean(selectedConversation && conversationHasInFlightSend(selectedConversation));

  // ── Load conversations on mount ─────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;
    async function run() {
      setStatus('正在加载会话');
      try {
        const response = await messageApi.getConversationSeqs([]);
        const states = response.states ?? response.conversations ?? response.seqs ?? [];
        const previews = states.map((s) => conversationStateToView(s, currentUserId, s.lastMessage ? [s.lastMessage] : []));
        const needsFriendProfiles = previews.some(
          (c) => c.chatType === 'single' && c.receiverId && c.title === UNKNOWN_CONTACT_LABEL,
        );
        const conversationsPromise = Promise.all(states.map((s) => loadConversation(s, currentUserId, messageApi, groupsApi)));
        const friendProfilesPromise = needsFriendProfiles
          ? loadAcceptedFriendProfileMap(contactsApi)
          : Promise.resolve(new Map<string, UserProfile>());

        if (!cancelled) {
          setItems((current) => mergeLoadedConversations(current, previews));
          setStatus(previews.length > 0 ? `已加载 ${previews.length} 个会话，正在同步消息` : '暂无会话');
        }

        void friendProfilesPromise.then((friendProfiles) => {
          if (!cancelled && friendProfiles.size > 0) {
            setItems((current) => hydrateConversationTitles(current, friendProfiles));
          }
        });

        const [conversations, friendProfiles] = await Promise.all([conversationsPromise, friendProfilesPromise]);
        if (!cancelled) {
          setItems((current) => mergeLoadedConversations(current, hydrateConversationTitles(conversations, friendProfiles)));
          setStatus(conversations.length > 0 ? `已加载 ${conversations.length} 个会话` : '暂无会话');
        }
      } catch (error) {
        if (!cancelled) setStatus(error instanceof Error ? error.message : '加载会话失败');
      }
    }
    void run();
    return () => { cancelled = true; };
  }, [currentUserId, messageApi, contactsApi, groupsApi]);

  // ── WebSocket connection ────────────────────────────────────────────────
  useEffect(() => {
    if (!webSocketUrl || (!webSocketToken && !webSocketFactory) || (!webSocketFactory && !canUseNativeWebSocket())) {
      return;
    }
    const client = createMessageWebSocketClient({
      url: webSocketUrl,
      token: webSocketToken,
      webSocketFactory,
      reconnect: true,
      onAuthFailure,
      onEvent: (event) => {
        const message = webSocketEventToServerMessage(event);
        if (!message || !conversationBelongsToCurrentUser(message, currentUserId)) return;
        setItems((current) => upsertLiveServerMessage(current, serverMessageToChatMessage(message, currentUserId)));
        setStatus('收到新消息');
      },
      onClose: () => {
        setStatus((current) => (current === '收到新消息' ? current : 'WebSocket 已断开，正在重连'));
      },
      onReconnecting: (_attempt, delayMs) => {
        setStatus(`正在重连 (${Math.round(delayMs / 1000)}s 后)`);
      },
      onOpen: () => {
        setStatus((current) =>
          current.startsWith('正在重连') || current === 'WebSocket 已断开，正在重连' ? '已重连' : current,
        );
      },
    });
    client.connect();
    return () => client.close(1000, 'messages page unmounted');
  }, [currentUserId, onAuthFailure, webSocketUrl, webSocketToken, webSocketFactory]);

  // ── Pending chat/group signals ──────────────────────────────────────────
  useEffect(() => {
    if (startChatSignal > 0) { setShowStartChat(true); setSelectedConversationId(null); }
  }, [startChatSignal]);

  useEffect(() => {
    if (!pendingChatProfile) return;
    handleStartChat(pendingChatProfile);
    onPendingChatConsumed?.();
  }, [pendingChatProfile, onPendingChatConsumed]);

  useEffect(() => {
    if (!pendingGroup) return;
    void handleOpenGroup(pendingGroup);
    onPendingGroupConsumed?.();
  }, [pendingGroup, onPendingGroupConsumed]);

  // ── Draft → loaded conversation promotion ───────────────────────────────
  useEffect(() => {
    if (!selectedConversationId?.startsWith('draft-single:')) return;
    const peerId = draftPeerId(selectedConversationId);
    const loaded = items.find((c) => c.id !== selectedConversationId && isSingleConversationWithPeer(c, peerId));
    if (loaded) setSelectedConversationId(loaded.id);
  }, [items, selectedConversationId]);

  // ── Mark incoming messages read ─────────────────────────────────────────
  useEffect(() => {
    if (!selectedConversation) return;
    const visibleMaxSeq = maxReadableSeq(selectedConversation.messages.filter((m) => m.direction === 'incoming'));
    if (visibleMaxSeq === undefined || visibleMaxSeq <= (selectedConversation.hasReadSeq ?? 0)) return;
    const syncKey = `${selectedConversation.id}:${visibleMaxSeq}`;
    if (readSyncsInFlight.current.has(syncKey)) return;
    readSyncsInFlight.current.add(syncKey);
    void messageApi
      .markRead(selectedConversation.id, { hasReadSeq: visibleMaxSeq })
      .then((response) => {
        const confirmedReadSeq = response.hasReadSeq ?? visibleMaxSeq;
        setItems((current) => markConversationRead(current, selectedConversation.id, confirmedReadSeq));
      })
      .catch((error) => { setStatus(error instanceof Error ? error.message : '标记已读失败'); })
      .finally(() => { readSyncsInFlight.current.delete(syncKey); });
  }, [messageApi, selectedConversation]);

  // ── AI hosting state ────────────────────────────────────────────────────
  useEffect(() => {
    if (!selectedConversation || !conversationSupportsAIHosting(selectedConversation)) return;
    let cancelled = false;
    const id = selectedConversation.id;
    setAIHostingByConversation((cur) => ({
      ...cur,
      [id]: { ...(cur[id] ?? { updating: false }), loading: true, error: '' },
    }));
    void messageApi
      .getAIHosting(id)
      .then((state) => {
        if (!cancelled) {
          setAIHostingByConversation((cur) => ({ ...cur, [id]: { state, loading: false, updating: false, error: '' } }));
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setAIHostingByConversation((cur) => ({
            ...cur,
            [id]: {
              ...(cur[id] ?? { updating: false }),
              loading: false,
              error: error instanceof Error ? error.message : 'AI 托管状态加载失败',
            },
          }));
        }
      });
    return () => { cancelled = true; };
  }, [messageApi, selectedConversation?.chatType, selectedConversation?.id]);

  const retryAIHosting = useCallback(
    (id: string) => {
      setAIHostingByConversation((cur) => ({
        ...cur,
        [id]: { ...(cur[id] ?? { updating: false }), loading: true, error: '' },
      }));
      void messageApi
        .getAIHosting(id)
        .then((state) => {
          setAIHostingByConversation((cur) => ({ ...cur, [id]: { state, loading: false, updating: false, error: '' } }));
        })
        .catch((error) => {
          setAIHostingByConversation((cur) => ({
            ...cur,
            [id]: {
              ...(cur[id] ?? { updating: false }),
              loading: false,
              error: error instanceof Error ? error.message : 'AI 托管状态加载失败',
            },
          }));
        });
    },
    [messageApi],
  );

  const toggleAIHosting = useCallback(
    (id: string, enabled: boolean) => {
      setAIHostingByConversation((cur) => ({
        ...cur,
        [id]: { ...(cur[id] ?? { loading: false, error: '' }), updating: true, error: '' },
      }));
      void messageApi
        .updateAIHosting(id, { enabled })
        .then((state) => {
          setAIHostingByConversation((cur) => ({ ...cur, [id]: { state, loading: false, updating: false, error: '' } }));
          setStatus(enabled ? 'AI 托管已开启' : 'AI 托管已关闭');
        })
        .catch((error) => {
          setAIHostingByConversation((cur) => ({
            ...cur,
            [id]: {
              ...(cur[id] ?? { loading: false }),
              updating: false,
              error: error instanceof Error ? error.message : 'AI 托管更新失败',
            },
          }));
        });
    },
    [messageApi],
  );

  // ── Event handlers ──────────────────────────────────────────────────────
  function handleStartChat(profile: Parameters<typeof userProfileToDraftConversation>[0]) {
    const draft = userProfileToDraftConversation(profile);
    const existing = items.find((c) => c.chatType === 'single' && c.receiverId === profile.user_id);
    setItems((current) => upsertStartedConversation(current, existing?.id, draft));
    setSelectedConversationId(existing?.id ?? draft.id);
    setShowStartChat(false);
    setStatus(`已打开 ${draft.title} 的聊天`);
  }

  async function handleOpenGroup(group: Group) {
    const draft = groupToConversation(group);
    setItems((current) => upsertStartedConversation(current, undefined, draft));
    setSelectedConversationId(draft.id);
    setShowStartChat(false);
    setStatus(`已打开 ${group.name} 的聊天`);
    try {
      const result = await groupsApi.listMembers(group.group_id);
      const displayNames = groupMemberDisplayNameMap(result.members ?? []);
      setItems((current) => hydrateGroupConversationMembers(current, group, displayNames));
    } catch (error) {
      setStatus(error instanceof Error ? error.message : '加载群成员失败');
    }
  }

  function handleSend(content: string) {
    if (!selectedConversation) return;
    if (conversationHasInFlightSend(selectedConversation)) { setStatus('上一条消息发送中'); return; }
    const pending = createPendingMessage(selectedConversation, { contentType: 'text', content }, currentUserId);
    setItems((current) => appendMessage(current, selectedConversation.id, pending));
    void Promise.resolve()
      .then(() => sendMessageWithApi(messageApi, pending))
      .then((sent) => {
        setItems((current) => confirmSentMessage(current, selectedConversation.id, pending.id, sent));
        setSelectedConversationId(sent.conversationId);
      })
      .catch((error) => {
        setStatus(sendErrorMessage(error, selectedConversation.chatType));
        setItems((current) => updateMessage(current, selectedConversation.id, pending.id, { ...pending, status: 'failed' }));
      });
  }

  const handleGroupManagementUpdated = useCallback((group: Group, members: GroupMember[]) => {
    setItems((current) => hydrateGroupConversationMembers(current, group, members));
  }, []);

  async function handleSendAttachment(file: File, kind: AttachmentKind) {
    if (!selectedConversation) return;
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
    let pendingMessage: ReturnType<typeof createPendingMessage> | null = null;

    setUploadingConversationId(conversationAtStart.id);
    setStatus(kind === 'image' ? '正在上传图片' : '正在上传文件');

    try {
      const dimensions = kind === 'image' ? await readImageDimensions(file) : undefined;
      const uploaded = await uploadFileToMedia({
        file,
        purpose: kind === 'image' ? 'message_image' : 'message_file',
        mediaApi,
        filename,
        contentType,
        ...(dimensions ?? {}),
      });
      const content =
        kind === 'image'
          ? JSON.stringify({ mediaId: uploaded.mediaId, filename, sizeBytes: file.size, contentType, ...(dimensions ?? {}) })
          : JSON.stringify({ mediaId: uploaded.mediaId, filename, sizeBytes: file.size, contentType });

      const nextPending = createPendingMessage(conversationAtStart, { contentType: kind, content }, currentUserId);
      pendingMessage = nextPending;
      setItems((current) => appendMessage(current, conversationAtStart.id, nextPending));

      const sent = await sendMessageWithApi(messageApi, nextPending);
      setItems((current) => confirmSentMessage(current, conversationAtStart.id, nextPending.id, sent));
      setSelectedConversationId(sent.conversationId);
      setStatus(kind === 'image' ? '图片已发送' : '文件已发送');
    } catch (error) {
      setStatus(error instanceof Error ? error.message : '发送附件失败');
      if (pendingMessage) {
        const failed = pendingMessage;
        setItems((current) => updateMessage(current, conversationAtStart.id, failed.id, { ...failed, status: 'failed' }));
      }
    } finally {
      setUploadingConversationId((current) => (current === conversationAtStart.id ? null : current));
    }
  }

  // ── Render ──────────────────────────────────────────────────────────────
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
        onBack={() => { setGroupManagementConversationId(null); setSelectedConversationId(null); }}
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
      onSelect={(id) => setSelectedConversationId(id)}
    />
  );
}
