import { ActionRow } from '../components/ui/ActionRow';
import { ListCard } from '../components/ui/ListCard';

type DiscoverEntry = {
  id: string;
  label: string;
  helper: string;
  accent: 'green' | 'blue' | 'purple' | 'orange' | 'gray';
  badge: string;
};

const discoverGroups: DiscoverEntry[][] = [
  [{ id: 'moments', label: '朋友圈', helper: '真实动态能力暂未接入', accent: 'green', badge: 'MVP 占位' }],
  [
    { id: 'scan', label: '扫一扫', helper: '暂不启动真实扫码', accent: 'blue', badge: 'MVP 占位' },
    { id: 'mini-programs', label: '小程序', helper: 'Agent 工具入口规划中', accent: 'purple', badge: 'MVP 占位' },
  ],
  [{ id: 'channels', label: '视频号', helper: '内容生态后续扩展', accent: 'orange', badge: 'MVP 占位' }],
];

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
