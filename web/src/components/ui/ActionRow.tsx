import { Bell, ChevronRight, MoreHorizontal, UsersRound } from 'lucide-react';
import type { ComponentType } from 'react';
import { Badge } from './Badge';
import { ListItem } from './ListItem';

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
    <ListItem
      className="action-row"
      leading={
        <div className={`action-icon action-${accent}`}>
          <LeadingIcon size={19} />
        </div>
      }
      headline={label}
      supportingText={helper}
      trailing={
        <>
          {badge ? <Badge className="row-badge">{badge}</Badge> : null}
          <TrailingIcon size={18} />
        </>
      }
    />
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
