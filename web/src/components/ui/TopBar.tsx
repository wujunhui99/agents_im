import { Plus, Search } from 'lucide-react';

type TopBarProps = {
  title: string;
};

export function TopBar({ title }: TopBarProps) {
  return (
    <header className="top-bar">
      <h1>{title}</h1>
      <div className="top-actions" aria-label="页面操作">
        <button type="button" aria-label="搜索">
          <Search size={20} />
        </button>
        <button type="button" aria-label="新增">
          <Plus size={21} />
        </button>
      </div>
    </header>
  );
}
