import { FileText, Image as ImageIcon, SendHorizontal } from 'lucide-react';
import { useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { Button } from '../../../components/ui/Button';
import { TextField } from '../../../components/ui/TextField';
import type { AttachmentKind, MentionTarget } from '../types';

type MemberOption = { userId: string; displayName: string };

export function SendMessageComposer({
  onSend,
  onSendAttachment,
  sending,
  mentionCandidates = [],
}: {
  onSend: (content: string, mentions: MentionTarget[]) => void;
  onSendAttachment: (file: File, kind: AttachmentKind) => void;
  sending: boolean;
  // 群成员候选（已排除自己）；非群聊传空数组 → 不启用 @ 选择。
  mentionCandidates?: MemberOption[];
}) {
  const [draft, setDraft] = useState('');
  // 已选中的 @ 成员（可能有重复的 displayName，发送时按文本里是否仍存在 @token 过滤）。
  const [selectedMentions, setSelectedMentions] = useState<MentionTarget[]>([]);
  const [pickerOpen, setPickerOpen] = useState(false);
  const trimmedDraft = draft.trim();

  const atEnabled = mentionCandidates.length > 0;

  // 发送时保留仍在文本里的 @token 对应的成员（用户删了 @xxx 就不再唤醒）。
  const activeMentions = useMemo(() => {
    return selectedMentions.filter((mention) => draft.includes(`@${mention.displayName}`));
  }, [selectedMentions, draft]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !trimmedDraft) return;
    onSend(trimmedDraft, activeMentions);
    setDraft('');
    setSelectedMentions([]);
    setPickerOpen(false);
  }

  function handleDraftChange(value: string) {
    setDraft(value);
    if (!atEnabled) return;
    // 末字符是 @ 时弹出成员选择。
    setPickerOpen(value.endsWith('@'));
  }

  function handlePickMember(member: MemberOption) {
    // 把光标处的触发 @ 替换成 "@昵称 "，并记录被 @ 成员。
    setDraft((current) => `${current.endsWith('@') ? current.slice(0, -1) : current}@${member.displayName} `);
    setSelectedMentions((current) => [...current, { userId: member.userId, displayName: member.displayName }]);
    setPickerOpen(false);
  }

  function handleAttachmentChange(event: ChangeEvent<HTMLInputElement>, kind: AttachmentKind) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = '';
    if (!file || sending) return;
    onSendAttachment(file, kind);
  }

  return (
    <form className="message-composer" aria-label="发送消息" onSubmit={handleSubmit}>
      <label className={`message-attachment-button${sending ? ' is-disabled' : ''}`} title="发送图片">
        <ImageIcon size={18} />
        <span className="sr-only">发送图片</span>
        <input
          className="sr-only"
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif,image/*"
          aria-label="发送图片"
          disabled={sending}
          onChange={(event) => handleAttachmentChange(event, 'image')}
        />
      </label>
      <label className={`message-attachment-button${sending ? ' is-disabled' : ''}`} title="发送文件">
        <FileText size={18} />
        <span className="sr-only">发送文件</span>
        <input
          className="sr-only"
          type="file"
          aria-label="发送文件"
          disabled={sending}
          onChange={(event) => handleAttachmentChange(event, 'file')}
        />
      </label>
      <div className="message-composer-input">
        {pickerOpen && atEnabled ? (
          <ul className="mention-picker" role="listbox" aria-label="选择要提醒的成员">
            {mentionCandidates.map((member) => (
              <li key={member.userId}>
                <button
                  type="button"
                  className="mention-picker-option"
                  role="option"
                  aria-selected={false}
                  onClick={() => handlePickMember(member)}
                >
                  @{member.displayName}
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        <TextField
          label="输入消息"
          hideLabel
          value={draft}
          placeholder={atEnabled ? '输入消息，@ 提醒群成员' : '输入消息'}
          disabled={sending}
          onChange={(event) => handleDraftChange(event.target.value)}
          fieldClassName="message-composer-field"
        />
      </div>
      <Button className="message-send-button" type="submit" disabled={sending || !trimmedDraft}>
        <SendHorizontal size={17} />
        <span>{sending ? '发送中' : '发送'}</span>
      </Button>
    </form>
  );
}
