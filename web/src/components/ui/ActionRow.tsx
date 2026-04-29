import { Bell, ChevronRight, MoreHorizontal, UsersRound } from 'lucide-react';
import type { ComponentType } from 'react';

type Accent = 'green' | 'blue' | 'purple' | 'orange' | 'gray';

type ActionRowProps = {
  label: string;
  helper: string;
  accent: Accent;
  badge?: string;
  icon?: ComponentType<{ size?: number }>;
  trailingIcon?: ComponentType<{ size?: number }>;
};

export function ActionRow({ label, helper, accent, badge, icon, trailingIcon: TrailingIcon = ChevronRight }: ActionRowProps) {
  const LeadingIcon = icon ?? defaultIconForAccent(accent);

  return (
    <article className="action-row">
      <div className={`action-icon action-${accent}`}>
        <LeadingIcon size={19} />
      </div>
      <div className="row-main">
        <strong>{label}</strong>
        <p>{helper}</p>
      </div>
      <div className="row-trailing">
        {badge ? <span className="row-badge">{badge}</span> : null}
        <TrailingIcon size={18} />
      </div>
    </article>
  );
}

function defaultIconForAccent(accent: Accent) {
  if (accent === 'gray') {
    return MoreHorizontal;
  }

  if (accent === 'green') {
    return UsersRound;
  }

  return Bell;
}
