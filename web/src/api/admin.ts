import { createApiClient, type ApiClient } from './client';

export type AdminDashboardTotals = {
  users: number;
  conversations: number;
  messages: number;
  aiRuns: number;
  failedAiRuns: number;
};

export type AdminMessage = {
  serverMsgId: string;
  clientMsgId: string;
  conversationId: string;
  seq: number;
  senderId: string;
  receiverId?: string;
  groupId?: string;
  chatType: string;
  contentType: string;
  content: string;
  messageOrigin: 'human' | 'ai' | 'system' | string;
  agentAccountId?: string;
  triggerServerMsgId?: string;
  agentRunId?: string;
  allowRecursiveTrigger?: boolean;
  sendTime: number;
  createdAt: number;
};

export type AdminConversation = {
  conversationId: string;
  maxSeq: number;
  hasReadSeq?: number;
  unreadCount?: number;
  maxSeqTime?: number;
  lastMessage?: AdminMessage;
};

export type AdminUser = {
  userId: string;
  accountId?: string;
  identifier: string;
  displayName: string;
  name: string;
  gender: string;
  birthDate: string;
  region: string;
  accountType: 'user' | 'agent' | 'admin' | string;
  avatarMediaId?: string;
  avatarUrl?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type AdminFriend = {
  userId: string;
  friendId: string;
  status: string;
  isFriend: boolean;
  friend?: AdminUser;
  createdAt: string;
  updatedAt: string;
};

export type AdminLLMTrace = {
  traceId: string;
  traceUrl?: string;
  runId: string;
  agentId: string;
  conversationId?: string;
  triggerMessageId?: string;
  responseMessageId?: string;
  requestingUserId?: string;
  status: string;
  provider?: string;
  model?: string;
  promptHash?: string;
  promptVersion?: string;
  latencyMs?: number;
  totalTokens?: number;
  errorCode?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt?: string;
};

export type AdminAgentToolCall = {
  toolCallId: string;
  runId: string;
  toolName: string;
  status: string;
  durationMs?: number;
  errorCode?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt?: string;
};

export type AdminAgentFileRead = {
  fileReadId: string;
  runId: string;
  skillId?: string;
  fileId?: string;
  status: string;
  byteCount?: number;
  errorCode?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt?: string;
};

export type AdminAgentPythonExec = {
  pythonExecId: string;
  runId: string;
  status: string;
  errorCode?: string;
  errorMessage?: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt?: string;
};

export type AdminDashboard = {
  totals: AdminDashboardTotals;
  recentTraces: AdminLLMTrace[];
  recentConversations: AdminConversation[];
};

export type AdminConversationMessagesResponse = {
  conversationId: string;
  messages: AdminMessage[];
  isEnd: boolean;
  nextSeq: number;
};

export type AdminUserSearchResponse = {
  users: AdminUser[];
};

export type AdminUserDetailResponse = {
  user: AdminUser;
};

export type AdminTestAccountCreateRequest = {
  identifier: string;
  displayName?: string;
  /** 缺省时由服务端生成，并在响应中一次性返回。 */
  password?: string;
};

export type AdminTestAccountCreateResponse = {
  user: AdminUser;
  /** 生效的登录密码，仅本次响应返回。 */
  password: string;
  /** identifier 已是 test 账户，本次操作为重置其密码。 */
  alreadyExisted: boolean;
};

export type AdminUserFriendsResponse = {
  friends: AdminFriend[];
};

export type AdminUserConversationsResponse = {
  conversations: AdminConversation[];
};

export type AdminLLMTraceListResponse = {
  traces: AdminLLMTrace[];
};

export type AdminLLMTraceDetailResponse = {
  trace: AdminLLMTrace;
  toolCalls: AdminAgentToolCall[];
  fileReads: AdminAgentFileRead[];
  pythonExecs: AdminAgentPythonExec[];
};

export type AdminFeedback = {
  feedbackId: string;
  userId: string;
  category: string;
  status: string;
  title: string;
  content: string;
  contact?: string;
  pageUrl?: string;
  userAgent?: string;
  clientMeta?: Record<string, unknown>;
  adminNote?: string;
  createdAt: string;
  updatedAt: string;
};

export type AdminFeedbackListRequest = {
  status?: string;
  limit?: number;
  offset?: number;
};

export type AdminFeedbackListResponse = {
  items: AdminFeedback[];
};

export type AdminFeedbackDetailResponse = {
  feedback: AdminFeedback;
};

export type AdminFeedbackUpdateRequest = {
  status: string;
  adminNote?: string;
};

export type AdminTaskReport = {
  taskId: string;
  agent: string;
  codexSessionId?: string;
  issueNumber?: number;
  issueUrl?: string;
  repo: string;
  branch?: string;
  worktree?: string;
  commit?: string;
  outcome: string;
  startedAt?: string;
  endedAt?: string;
  durationSeconds?: number;
  tokensUsed?: number;
  prUrl?: string;
  evidence: string[];
  blockers: string[];
  majorTimeSinks: string[];
  wouldMorePermissionHelp?: string;
  candidatePermissions: string[];
  permissionReason?: string;
  pitfallsOrLessons: string[];
  notes?: string;
  recordedAt: string;
};

export type AdminTaskReportListRequest = {
  outcome?: string;
  limit?: number;
  offset?: number;
};

export type AdminTaskReportListResponse = {
  items: AdminTaskReport[];
};

export type AdminTaskReportDetailResponse = {
  report: AdminTaskReport;
};

export type AdminApi = {
  getDashboard: () => Promise<AdminDashboard>;
  listLLMTraces: () => Promise<AdminLLMTraceListResponse>;
  getLLMTraceDetail: (traceId: string) => Promise<AdminLLMTraceDetailResponse>;
  getConversationMessages: (conversationId: string) => Promise<AdminConversationMessagesResponse>;
  searchUsers: (query: string) => Promise<AdminUserSearchResponse>;
  createTestAccount: (request: AdminTestAccountCreateRequest) => Promise<AdminTestAccountCreateResponse>;
  getUserDetail: (accountId: string) => Promise<AdminUserDetailResponse>;
  getUserFriends: (accountId: string) => Promise<AdminUserFriendsResponse>;
  getUserConversations: (accountId: string) => Promise<AdminUserConversationsResponse>;
  listFeedback: (request?: AdminFeedbackListRequest) => Promise<AdminFeedbackListResponse>;
  getFeedback: (feedbackId: string) => Promise<AdminFeedbackDetailResponse>;
  updateFeedback: (feedbackId: string, request: AdminFeedbackUpdateRequest) => Promise<AdminFeedbackDetailResponse>;
  listTaskReports: (request?: AdminTaskReportListRequest) => Promise<AdminTaskReportListResponse>;
};

export function createAdminApi(api: ApiClient = createApiClient()): AdminApi {
  const feedbackBasePath = '/api/admin/feedback';
  const taskReportsBasePath = '/api/admin/task-reports';

  return {
    getDashboard() {
      return api.get<AdminDashboard>('/admin/dashboard');
    },
    listLLMTraces() {
      return api.get<AdminLLMTraceListResponse>('/admin/llm-traces');
    },
    getLLMTraceDetail(traceId) {
      return api.get<AdminLLMTraceDetailResponse>(`/admin/llm-traces/${encodeURIComponent(traceId)}`);
    },
    getConversationMessages(conversationId) {
      const params = new URLSearchParams({ fromSeq: '1', limit: '200', order: 'asc' });
      return api.get<AdminConversationMessagesResponse>(
        `/admin/conversations/${encodeURIComponent(conversationId)}/messages?${params.toString()}`,
      );
    },
    searchUsers(query) {
      const params = new URLSearchParams({ query, limit: '20' });
      return api.get<AdminUserSearchResponse>(`/admin/users?${params.toString()}`);
    },
    createTestAccount(request) {
      return api.post<AdminTestAccountCreateResponse>('/admin/test-accounts', request);
    },
    getUserDetail(accountId) {
      return api.get<AdminUserDetailResponse>(`/admin/users/${encodeURIComponent(accountId)}`);
    },
    getUserFriends(accountId) {
      return api.get<AdminUserFriendsResponse>(`/admin/users/${encodeURIComponent(accountId)}/friends`);
    },
    getUserConversations(accountId) {
      return api.get<AdminUserConversationsResponse>(`/admin/users/${encodeURIComponent(accountId)}/conversations`);
    },
    listFeedback(request = {}) {
      const params = new URLSearchParams();
      if (request.status) params.set('status', request.status);
      if (request.limit !== undefined) params.set('limit', String(request.limit));
      if (request.offset !== undefined) params.set('offset', String(request.offset));
      const query = params.toString();
      return api.get<AdminFeedbackListResponse>(`${feedbackBasePath}${query ? `?${query}` : ''}`);
    },
    getFeedback(feedbackId) {
      return api.get<AdminFeedbackDetailResponse>(`${feedbackBasePath}/${encodeURIComponent(feedbackId)}`);
    },
    updateFeedback(feedbackId, request) {
      return api.patch<AdminFeedbackDetailResponse>(`${feedbackBasePath}/${encodeURIComponent(feedbackId)}`, request);
    },
    listTaskReports(request = {}) {
      const params = new URLSearchParams();
      if (request.outcome) params.set('outcome', request.outcome);
      if (request.limit !== undefined) params.set('limit', String(request.limit));
      if (request.offset !== undefined) params.set('offset', String(request.offset));
      const query = params.toString();
      return api.get<AdminTaskReportListResponse>(`${taskReportsBasePath}${query ? `?${query}` : ''}`);
    },
  };
}
