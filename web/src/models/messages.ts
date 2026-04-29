export type ChatType = 'single' | 'group';
export type MessageContentType = 'text';
export type MessageStatus = 'sending' | 'sent' | 'failed';
export type MessageDirection = 'incoming' | 'outgoing';
export type ConversationAccent = 'green' | 'blue' | 'purple' | 'orange' | 'gray';

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
  time: string;
  unread: number;
  color: ConversationAccent;
  chatType: ChatType;
  receiverId?: string;
  groupId?: string;
  messages: ChatMessage[];
};
