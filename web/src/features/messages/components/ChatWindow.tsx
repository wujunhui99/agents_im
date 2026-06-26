import { ChevronLeft, FileText, Image as ImageIcon } from 'lucide-react';
import { Fragment, useLayoutEffect, useMemo, useRef, type ReactNode } from 'react';
import type { MediaApi } from '../../../api/media';
import { Avatar } from '../../../components/ui/Avatar';
import { Button } from '../../../components/ui/Button';
import { MessageBubble } from '../../../components/ui/MessageBubble';
import type { ChatMessage, Conversation } from '../../../models/messages';
import type { AIHostingPanelState, AttachmentKind, MediaDownloadHandler } from '../types';
import { authoritativeSeq, formatMessageDate, formatMessageTime, orderedChatMessages, shouldRenderDateSeparator } from '../utils/messageOrdering';
import { messageDisplayText } from '../utils/mediaUtils';
import { conversationSupportsAIHosting } from '../utils/conversationUtils';
import { AIHostingControl } from './AIHostingControl';
import { FileMessageBubble, ImageMessageBubble } from './MessageBubbles';
import { SendMessageComposer } from './SendMessageComposer';

const statusLabels: Record<string, string> = {
  sending: '发送中',
  failed: '发送失败',
};

function renderMessageMetadata(message: ChatMessage, hasReadSeq: number | undefined): ReactNode {
  return (
    <span className="message-metadata">
      <time className="message-time" dateTime={new Date(message.sendTime).toISOString()}>
        {formatMessageTime(message.sendTime)}
      </time>
      {message.direction === 'outgoing' ? renderOutgoingMessageStatus(message, hasReadSeq) : null}
    </span>
  );
}

function renderOutgoingMessageStatus(message: ChatMessage, hasReadSeq: number | undefined): ReactNode {
  if (message.status === 'sent') {
    // seq=0 是 Kafka 写路径的占位 ACK（异步分配，03 §9 B2），不得参与已读比较。
    const seq = authoritativeSeq(message);
    const read = seq !== undefined && seq <= (hasReadSeq ?? 0);
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

function renderMessageContent(message: ChatMessage): ReactNode {
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

function messageAriaLabel(message: ChatMessage) {
  const label = messageDisplayText(message);
  if (message.messageOrigin === 'ai') return `AI Agent 消息：${label}`;
  if (message.messageOrigin === 'system') return `系统消息：${label}`;
  return message.direction === 'outgoing' ? `我发送的消息：${label}` : `收到的消息：${label}`;
}

function MessageDateSeparator({ timestamp }: { timestamp: number }) {
  const label = formatMessageDate(timestamp);
  return (
    <div className="message-date-separator" role="separator" aria-label={label}>
      <span>{label}</span>
    </div>
  );
}

export function ChatWindow({
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
  const messageThreadRef = useRef<HTMLDivElement>(null);
  const latestMessage = sortedMessages[sortedMessages.length - 1];
  const latestMessageScrollKey = latestMessage
    ? [
        conversation.id,
        sortedMessages.length,
        latestMessage.id,
        latestMessage.serverMsgId ?? '',
        latestMessage.seq ?? '',
        latestMessage.status,
      ].join(':')
    : `${conversation.id}:empty`;

  useLayoutEffect(() => {
    const messageThread = messageThreadRef.current;
    if (!messageThread) return;
    messageThread.scrollTop = messageThread.scrollHeight;
  }, [latestMessageScrollKey]);

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
      <p className="inline-status" role="status">{status}</p>
      <div
        className="message-thread"
        role="log"
        aria-label="聊天消息"
        data-testid="message-thread-scroll-region"
        ref={messageThreadRef}
      >
        {sortedMessages.map((message, index) => (
          <Fragment key={message.id}>
            {shouldRenderDateSeparator(message, sortedMessages[index - 1]) ? <MessageDateSeparator timestamp={message.sendTime} /> : null}
            <article
              className={`message-row message-${message.direction} message-origin-${message.messageOrigin}`}
              aria-label={messageAriaLabel(message)}
            >
              <div className="message-body">
                {conversation.chatType === 'group' && message.direction === 'incoming' ? (
                  <span className="message-sender-name">{message.senderDisplayName ?? '群成员'}</span>
                ) : null}
                {message.messageOrigin === 'ai' ? <span className="message-origin-badge">AI Agent</span> : null}
                {message.messageOrigin === 'system' ? <span className="message-origin-badge message-origin-system">系统</span> : null}
                {message.contentType === 'image' ? (
                  <ImageMessageBubble
                    message={message}
                    mediaApi={mediaApi}
                    downloadMedia={downloadMedia}
                    onStatus={onStatus}
                    metadata={renderMessageMetadata(message, conversation.hasReadSeq)}
                  />
                ) : message.contentType === 'file' ? (
                  <FileMessageBubble
                    message={message}
                    mediaApi={mediaApi}
                    downloadMedia={downloadMedia}
                    onStatus={onStatus}
                    metadata={renderMessageMetadata(message, conversation.hasReadSeq)}
                  />
                ) : (
                  <MessageBubble direction={message.direction} metadata={renderMessageMetadata(message, conversation.hasReadSeq)}>
                    {renderMessageContent(message)}
                  </MessageBubble>
                )}
              </div>
            </article>
          </Fragment>
        ))}
      </div>
      <SendMessageComposer onSend={onSend} onSendAttachment={onSendAttachment} sending={sending} />
    </section>
  );
}
