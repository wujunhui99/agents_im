import type { ReactNode } from 'react';

type MessageBubbleProps = {
  children: ReactNode;
  direction: 'incoming' | 'outgoing';
  status?: ReactNode;
};

export function MessageBubble({ children, direction, status }: MessageBubbleProps) {
  return (
    <div className={`md-message-bubble md-message-bubble-${direction}`}>
      <p>{children}</p>
      {status ? <span className="md-message-bubble-status">{status}</span> : null}
    </div>
  );
}
