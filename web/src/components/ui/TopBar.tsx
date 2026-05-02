import { Plus, Search } from 'lucide-react';
import { Button } from './Button';
import { TopAppBar } from './TopAppBar';

type TopBarProps = {
  title: string;
  onAdd?: () => void;
};

export function TopBar({ title, onAdd }: TopBarProps) {
  return (
    <TopAppBar
      title={title}
      actions={
        <>
          <Button variant="icon" aria-label="搜索">
            <Search size={20} />
          </Button>
          <Button variant="icon" aria-label="新增" disabled={!onAdd} onClick={onAdd}>
            <Plus size={21} />
          </Button>
        </>
      }
    />
  );
}
