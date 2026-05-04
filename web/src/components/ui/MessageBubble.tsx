import type { ReactNode } from 'react';

type MessageBubbleProps = {
  children: ReactNode;
  direction: 'incoming' | 'outgoing';
  status?: ReactNode;
};

export function MessageBubble({ children, direction, status }: MessageBubbleProps) {
  const textClassName = status ? 'md-message-bubble-text md-message-bubble-text-with-status' : 'md-message-bubble-text';

  return (
    <div className={`md-message-bubble md-message-bubble-${direction}`}>
      <p className={textClassName}>
        <span className="md-message-bubble-content">{children}</span>
        {status ? <span className="md-message-bubble-status">{status}</span> : null}
      </p>
    </div>
  );
}
