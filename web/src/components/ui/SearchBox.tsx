import { Search } from 'lucide-react';
import { TextField } from './TextField';

type SearchBoxProps = {
  placeholder: string;
};

export function SearchBox({ placeholder }: SearchBoxProps) {
  return <TextField label={placeholder} hideLabel placeholder={placeholder} leadingIcon={<Search size={17} />} fieldClassName="search-box" />;
}
