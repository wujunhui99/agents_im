import { ActionRow } from '../components/ui/ActionRow';
import { ListCard } from '../components/ui/ListCard';
import { discoverGroups } from '../data/mockData';

export function DiscoverPage() {
  return (
    <div className="page-stack discover-page">
      {discoverGroups.map((group) => (
        <ListCard key={group.map((item) => item.id).join('-')}>
          {group.map((item) => (
            <ActionRow key={item.id} label={item.label} helper={item.helper} accent={item.accent} badge={item.badge} />
          ))}
        </ListCard>
      ))}
    </div>
  );
}
