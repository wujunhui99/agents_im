export type ChatType = 'single' | 'group';
export type MessageContentType = 'text';
export type MessageStatus = 'sending' | 'sent' | 'failed';
export type MessageDirection = 'incoming' | 'outgoing';
export type ConversationAccent = 'green' | 'blue' | 'purple' | 'orange' | 'gray';
export type MessageOrigin = 'human' | 'ai' | 'system';

export type ChatMessage = {
  id: string;
  conversationId: string;
  clientMsgId?: string;
  serverMsgId?: string;
  seq?: number;
  senderId: string;
  receiverId?: string;
  groupId?: string;
  chatType: ChatType;
  contentType: MessageContentType;
  content: string;
  messageOrigin: MessageOrigin;
  agentAccountId?: string;
  triggerServerMsgId?: string;
  agentRunId?: string;
  allowRecursiveTrigger?: boolean;
  sendTime: number;
  createdAt?: number;
  direction: MessageDirection;
  status: MessageStatus;
};

export type Conversation = {
  id: string;
  title: string;
  avatar: string;
  preview: string;
  previewOrigin?: MessageOrigin;
  time: string;
  unread: number;
  maxSeq?: number;
  hasReadSeq?: number;
  color: ConversationAccent;
  chatType: ChatType;
  receiverId?: string;
  groupId?: string;
  messages: ChatMessage[];
};
