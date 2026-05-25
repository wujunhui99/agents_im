import { ArrowLeft, Plus, Search } from 'lucide-react';
import { Button } from './Button';
import { TopAppBar } from './TopAppBar';

type TopBarProps = {
  title: string;
  onAdd?: () => void;
  onBack?: () => void;
};

export function TopBar({ title, onAdd, onBack }: TopBarProps) {
  return (
    <TopAppBar
      title={title}
      navigation={
        onBack ? (
          <Button variant="icon" aria-label="返回" onClick={onBack}>
            <ArrowLeft size={21} />
          </Button>
        ) : undefined
      }
      actions={
        onBack ? undefined : (
          <>
            <Button variant="icon" aria-label="搜索">
              <Search size={20} />
            </Button>
            <Button variant="icon" aria-label="新增" disabled={!onAdd} onClick={onAdd}>
              <Plus size={21} />
            </Button>
          </>
        )
      }
    />
  );
}
