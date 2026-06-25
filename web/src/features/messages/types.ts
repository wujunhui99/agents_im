import type { AIHostingState, MessageApi } from '../../api/messages';
import type { ContactsApi, Friendship } from '../../api/contacts';
import type { Group, GroupsApi } from '../../api/groups';
import type { MediaApi } from '../../api/media';
import type { WebSocketFactory } from '../../api/websocketClient';
import type { UserApi, UserProfile } from '../../api/user';
import type { MessageContentType, MessageStatus } from '../../models/messages';

export type MessagesPageProps = {
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
  onAuthFailure?: (failure: unknown) => void;
  startChatSignal?: number;
  pendingChatProfile?: UserProfile | null;
  pendingGroup?: Group | null;
  onPendingChatConsumed?: () => void;
  onPendingGroupConsumed?: () => void;
};

export type AttachmentKind = 'image' | 'file';

export type PendingMessageInput = {
  contentType: MessageContentType;
  content: string;
};

export type AIHostingPanelState = {
  state?: AIHostingState;
  loading: boolean;
  updating: boolean;
  error: string;
};

export type ImageDimensions = {
  width: number;
  height: number;
};

export type MediaDownloadHandler = (downloadUrl: string, filename: string) => void;

export type ImageMessagePayload = {
  mediaId?: string;
  filename?: string;
  width?: number;
  height?: number;
  sizeBytes?: number;
  contentType?: string;
};

export type FileMessagePayload = {
  mediaId?: string;
  filename?: string;
  sizeBytes?: number;
  contentType?: string;
};

export const IMAGE_MAX_BYTES = 15 * 1024 * 1024;
export const FILE_MAX_BYTES = 20 * 1024 * 1024;
export const FALLBACK_CONTENT_TYPE = 'application/octet-stream';
export const allowedImageMimeTypes = new Set(['image/jpeg', 'image/png', 'image/webp', 'image/gif']);

export const statusLabels: Record<Exclude<MessageStatus, 'sent'>, string> = {
  sending: '发送中',
  failed: '发送失败',
};

// Re-export API types used across sub-modules
export type { AIHostingState, Friendship, Group, UserProfile };
