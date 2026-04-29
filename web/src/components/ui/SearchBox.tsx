import { Search } from 'lucide-react';

type SearchBoxProps = {
  placeholder: string;
};

export function SearchBox({ placeholder }: SearchBoxProps) {
  return (
    <label className="search-box">
      <Search size={17} />
      <input placeholder={placeholder} aria-label={placeholder} />
    </label>
  );
}
