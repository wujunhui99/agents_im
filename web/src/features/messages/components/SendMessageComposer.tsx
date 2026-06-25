import { FileText, Image as ImageIcon, SendHorizontal } from 'lucide-react';
import { useState, type ChangeEvent, type FormEvent } from 'react';
import { Button } from '../../../components/ui/Button';
import { TextField } from '../../../components/ui/TextField';
import type { AttachmentKind } from '../types';

export function SendMessageComposer({
  onSend,
  onSendAttachment,
  sending,
}: {
  onSend: (content: string) => void;
  onSendAttachment: (file: File, kind: AttachmentKind) => void;
  sending: boolean;
}) {
  const [draft, setDraft] = useState('');
  const trimmedDraft = draft.trim();

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || !trimmedDraft) return;
    onSend(trimmedDraft);
    setDraft('');
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
      <TextField
        label="输入消息"
        hideLabel
        value={draft}
        placeholder="输入消息"
        disabled={sending}
        onChange={(event) => setDraft(event.target.value)}
        fieldClassName="message-composer-field"
      />
      <Button className="message-send-button" type="submit" disabled={sending || !trimmedDraft}>
        <SendHorizontal size={17} />
        <span>{sending ? '发送中' : '发送'}</span>
      </Button>
    </form>
  );
}
