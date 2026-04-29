import { ActionRow } from '../components/ui/ActionRow';
import { Avatar } from '../components/ui/Avatar';
import { ListCard } from '../components/ui/ListCard';
import { SearchBox } from '../components/ui/SearchBox';
import { contacts, friends } from '../data/mockData';

export function ContactsPage() {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索联系人、账号或群聊" />
      <ListCard ariaLabel="联系人快捷入口">
        {contacts.map((item) => (
          <ActionRow key={item.id} label={item.label} helper={item.helper} accent={item.accent} />
        ))}
      </ListCard>
      <p className="section-label">A</p>
      <ListCard ariaLabel="好友列表">
        {friends.map((friend) => (
          <article className="friend-row" key={friend.id}>
            <Avatar label={friend.initial} color="blue" />
            <div>
              <strong>{friend.name}</strong>
              <p>{friend.identifier}</p>
            </div>
          </article>
        ))}
      </ListCard>
    </div>
  );
}
