import type { UserProfile } from '../api/user';

export const mockCurrentUser: UserProfile = {
  user_id: 'usr_000001',
  identifier: 'alice_001',
  display_name: 'junhui',
  name: 'junhui',
  gender: 'female',
  age: 30,
  region: 'Shanghai',
  created_at: '2026-04-29T12:00:00Z',
  updated_at: '2026-04-29T12:00:00Z',
};

export const conversations = [
  {
    id: 'product-room',
    title: '产品讨论群',
    avatar: '产',
    preview: '后端 MVP 已发布，开始搭前端主框架。',
    time: '20:08',
    unread: 3,
    color: 'green',
  },
  {
    id: 'junhui',
    title: 'junhui',
    avatar: 'J',
    preview: '参考微信，先做四个主页面。',
    time: '19:46',
    unread: 1,
    color: 'blue',
  },
  {
    id: 'agent',
    title: 'Agent 助手',
    avatar: 'AI',
    preview: '我可以帮你整理联系人和群聊。',
    time: '昨天',
    unread: 0,
    color: 'purple',
  },
] as const;

export const contacts = [
  { id: 'new', label: '新的朋友', helper: '好友申请与推荐', accent: 'orange' },
  { id: 'groups', label: '群聊', helper: '产品讨论群、Agent 群', accent: 'green' },
  { id: 'tags', label: '标签', helper: '按角色整理联系人', accent: 'blue' },
  { id: 'official', label: '公众号', helper: '系统通知与服务号', accent: 'gray' },
] as const;

export const friends = [
  { id: 'alice', name: 'Alice Chen', identifier: 'alice_001', initial: 'A' },
  { id: 'bob', name: 'Bob Lin', identifier: 'bob_002', initial: 'B' },
  { id: 'agent', name: 'Agent 助手', identifier: 'agent_helper', initial: 'AI' },
] as const;

export const discoverGroups = [
  [{ id: 'moments', label: '朋友圈', helper: '动态、图片和 Agent 工作流分享', accent: 'blue', badge: 'MVP 占位' }],
  [
    { id: 'scan', label: '扫一扫', helper: '暂不启动真实扫码', accent: 'green', badge: 'MVP 占位' },
    { id: 'shake', label: '摇一摇', helper: '后续探索式匹配入口', accent: 'purple', badge: 'MVP 占位' },
  ],
  [{ id: 'mini-programs', label: '小程序', helper: 'Agent 插件和工具入口', accent: 'orange', badge: 'MVP 占位' }],
] as const;
