import type { MentionTarget } from '../types';

// AtMessagePayload 是 `at`（@ 提醒）消息 content 的结构，对齐后端/OpenIM AtText 语义：
// 真正决定被 @ 的是 atUserList，text 仅用于展示；atUsersInfo 携带展示昵称供高亮渲染。
export type AtUserInfo = { atUserID: string; groupNickname?: string };
export type AtMessagePayload = {
  text: string;
  atUserList: string[];
  atUsersInfo?: AtUserInfo[];
  isAtSelf?: boolean;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function parseAtMessagePayload(content: string): AtMessagePayload | null {
  try {
    const parsed = JSON.parse(content) as unknown;
    if (isRecord(parsed) && typeof parsed.text === 'string' && Array.isArray(parsed.atUserList)) {
      const atUserList = parsed.atUserList.filter((id): id is string => typeof id === 'string');
      const atUsersInfo = Array.isArray(parsed.atUsersInfo)
        ? parsed.atUsersInfo
            .filter(isRecord)
            .filter((info): info is Record<string, unknown> => typeof info.atUserID === 'string')
            .map((info) => ({
              atUserID: info.atUserID as string,
              groupNickname: typeof info.groupNickname === 'string' ? info.groupNickname : undefined,
            }))
        : undefined;
      return {
        text: parsed.text,
        atUserList,
        atUsersInfo,
        isAtSelf: typeof parsed.isAtSelf === 'boolean' ? parsed.isAtSelf : undefined,
      };
    }
  } catch {
    return null;
  }
  return null;
}

// atMessageDisplayText 取 `at` 消息的展示文本；解析失败回退原文（用于会话预览 / aria label）。
export function atMessageDisplayText(content: string): string {
  return parseAtMessagePayload(content)?.text ?? content;
}

// buildAtMessageContent 把展示文本 + 被 @ 成员打包成 `at` content JSON（发送用）。去重后
// atUserList 只含 id、atUsersInfo 带展示昵称，供接收端高亮。
export function buildAtMessageContent(text: string, mentions: MentionTarget[]): string {
  const seen = new Set<string>();
  const unique: MentionTarget[] = [];
  for (const mention of mentions) {
    const userId = mention.userId.trim();
    if (!userId || seen.has(userId)) continue;
    seen.add(userId);
    unique.push({ userId, displayName: mention.displayName });
  }
  return JSON.stringify({
    text,
    atUserList: unique.map((mention) => mention.userId),
    atUsersInfo: unique.map((mention) => ({ atUserID: mention.userId, groupNickname: mention.displayName })),
    isAtSelf: false,
  });
}
