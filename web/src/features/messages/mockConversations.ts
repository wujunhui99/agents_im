import type { ChatMessage, Conversation } from '../../models/messages';

export const currentUserId = 'usr_000001';

const productMessages: ChatMessage[] = [
  {
    id: 'msg-product-1',
    conversationId: 'group:grp_product',
    serverMsgId: 'msg_000101',
    seq: 1,
    senderId: 'usr_000002',
    groupId: 'grp_product',
    chatType: 'group',
    contentType: 'text',
    content: '后端 MVP 已发布，开始搭前端主框架。',
    sendTime: 1777464000000,
    createdAt: 1777464000000,
    direction: 'incoming',
    status: 'sent',
  },
  {
    id: 'msg-product-2',
    conversationId: 'group:grp_product',
    serverMsgId: 'msg_000102',
    seq: 2,
    senderId: currentUserId,
    groupId: 'grp_product',
    chatType: 'group',
    contentType: 'text',
    content: '消息页先保留 mock，接口边界按 contract 对齐。',
    sendTime: 1777464300000,
    createdAt: 1777464300000,
    direction: 'outgoing',
    status: 'sent',
  },
];

const junhuiMessages: ChatMessage[] = [
  {
    id: 'msg-junhui-1',
    conversationId: 'single:usr_000001:usr_000002',
    serverMsgId: 'msg_000201',
    seq: 1,
    senderId: 'usr_000002',
    receiverId: currentUserId,
    chatType: 'single',
    contentType: 'text',
    content: '参考微信，先做四个主页面。',
    sendTime: 1777463100000,
    createdAt: 1777463100000,
    direction: 'incoming',
    status: 'sent',
  },
];

const agentMessages: ChatMessage[] = [
  {
    id: 'msg-agent-1',
    conversationId: 'single:usr_000001:agent_helper',
    serverMsgId: 'msg_000301',
    seq: 1,
    senderId: 'agent_helper',
    receiverId: currentUserId,
    chatType: 'single',
    contentType: 'text',
    content: '我可以帮你整理联系人和群聊。',
    sendTime: 1777377600000,
    createdAt: 1777377600000,
    direction: 'incoming',
    status: 'sent',
  },
  {
    id: 'msg-agent-2',
    conversationId: 'single:usr_000001:agent_helper',
    clientMsgId: 'web-mock-failed',
    senderId: currentUserId,
    receiverId: 'agent_helper',
    chatType: 'single',
    contentType: 'text',
    content: '同步最近的群聊摘要。',
    sendTime: 1777377900000,
    direction: 'outgoing',
    status: 'failed',
  },
];

export const mockConversations: Conversation[] = [
  {
    id: 'group:grp_product',
    title: '产品讨论群',
    avatar: '产',
    preview: '后端 MVP 已发布，开始搭前端主框架。',
    time: '20:08',
    unread: 3,
    color: 'green',
    chatType: 'group',
    groupId: 'grp_product',
    messages: productMessages,
  },
  {
    id: 'single:usr_000001:usr_000002',
    title: 'junhui',
    avatar: 'J',
    preview: '参考微信，先做四个主页面。',
    time: '19:46',
    unread: 1,
    color: 'blue',
    chatType: 'single',
    receiverId: 'usr_000002',
    messages: junhuiMessages,
  },
  {
    id: 'single:usr_000001:agent_helper',
    title: 'Agent 助手',
    avatar: 'AI',
    preview: '我可以帮你整理联系人和群聊。',
    time: '昨天',
    unread: 0,
    color: 'purple',
    chatType: 'single',
    receiverId: 'agent_helper',
    messages: agentMessages,
  },
];

export function cloneMockConversations() {
  return mockConversations.map((conversation) => ({
    ...conversation,
    messages: conversation.messages.map((message) => ({ ...message })),
  }));
}
