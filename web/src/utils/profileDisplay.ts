import type { UserProfile } from '../api/user';

export const UNKNOWN_CONTACT_LABEL = '未知联系人';
export const UNKNOWN_ACCOUNT_TYPE_LABEL = '账号类型未同步';

export function profileDisplayName(profile: Pick<UserProfile, 'display_name' | 'name' | 'identifier'>) {
  return firstNonEmpty(profile.display_name, profile.name, profile.identifier) ?? UNKNOWN_CONTACT_LABEL;
}

export function profileIdentifier(profile: Pick<UserProfile, 'identifier'>) {
  return firstNonEmpty(profile.identifier);
}

export function accountTypeLabel(accountType?: UserProfile['account_type']) {
  if (accountType === 'agent') {
    return 'Agent';
  }
  if (accountType === 'admin') {
    return '管理员';
  }
  if (accountType === 'user') {
    return '用户';
  }
  return UNKNOWN_ACCOUNT_TYPE_LABEL;
}

export function avatarText(value: string, fallback = UNKNOWN_CONTACT_LABEL) {
  return (value.trim() || fallback).slice(0, 2).toUpperCase();
}

export function firstNonEmpty(...values: Array<string | undefined | null>) {
  return values.map((value) => value?.trim()).find((value): value is string => Boolean(value));
}
