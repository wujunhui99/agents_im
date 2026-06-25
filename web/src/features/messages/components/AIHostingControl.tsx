import { Bot, RefreshCw } from 'lucide-react';
import { Button } from '../../../components/ui/Button';
import type { AIHostingPanelState } from '../types';

export function AIHostingControl({
  hosting,
  onToggle,
  onRetry,
}: {
  hosting?: AIHostingPanelState;
  onToggle: (enabled: boolean) => void;
  onRetry: () => void;
}) {
  const state = hosting?.state;
  const loading = hosting?.loading ?? false;
  const updating = hosting?.updating ?? false;
  const checked = Boolean(state?.enabled);
  const available = state?.available ?? true;
  const disabled = loading || updating || !available;
  const helperText =
    state?.unavailableReason ||
    hosting?.error ||
    (loading ? '正在加载 AI 托管状态' : checked ? '已开启，对方发来消息时自动代你回复' : '已关闭');

  return (
    <section className="ai-hosting-control" aria-label="AI 托管设置">
      <label className="ai-hosting-toggle">
        <span className="ai-hosting-label">
          <Bot size={17} />
          <span>AI 托管</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          aria-label="AI 托管"
          checked={checked}
          disabled={disabled}
          onChange={(event) => onToggle(event.currentTarget.checked)}
        />
      </label>
      <div className="ai-hosting-status-line">
        <span className={hosting?.error || state?.unavailableReason ? 'ai-hosting-warning' : ''}>{helperText}</span>
        {hosting?.error ? (
          <Button className="ai-hosting-retry" variant="text" size="small" type="button" onClick={onRetry} aria-label="重试 AI 托管状态">
            <RefreshCw size={14} />
            <span>重试</span>
          </Button>
        ) : null}
      </div>
    </section>
  );
}
