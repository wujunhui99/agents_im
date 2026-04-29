import { Avatar } from '../components/ui/Avatar';
import { ListCard } from '../components/ui/ListCard';
import { SearchBox } from '../components/ui/SearchBox';
import { conversations } from '../data/mockData';

export function MessagesPage() {
  return (
    <div className="page-stack">
      <SearchBox placeholder="搜索" />
      <ListCard ariaLabel="消息列表" className="conversation-list">
        {conversations.map((item) => (
          <article className="conversation-row" key={item.id}>
            <Avatar label={item.avatar} color={item.color} />
            <div className="row-main">
              <div className="row-title-line">
                <strong>{item.title}</strong>
                <time>{item.time}</time>
              </div>
              <p>{item.preview}</p>
            </div>
            {item.unread > 0 ? <span className="unread-badge">{item.unread}</span> : null}
          </article>
        ))}
      </ListCard>
    </div>
  );
}
