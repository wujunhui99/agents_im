import type { ReactNode } from 'react';

type MessageBubbleProps = {
  children: ReactNode;
  direction: 'incoming' | 'outgoing';
  metadata?: ReactNode;
};

export function MessageBubble({ children, direction, metadata }: MessageBubbleProps) {
  const textClassName = metadata ? 'md-message-bubble-text md-message-bubble-text-with-status' : 'md-message-bubble-text';

  return (
    <div className={`md-message-bubble md-message-bubble-${direction}`}>
      <p className={textClassName}>
        <span className="md-message-bubble-content">{children}</span>
        {metadata ? <span className="md-message-bubble-metadata">{metadata}</span> : null}
      </p>
    </div>
  );
}
