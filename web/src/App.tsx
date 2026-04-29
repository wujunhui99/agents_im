import { useMemo, useState } from 'react';
import { Compass, Contact, MessageCircle, UserRound } from 'lucide-react';
import { defaultUserApi, type UserApi, type UserProfile, type UserProfilePatch } from './api/user';
import { TabBar, type TabDefinition } from './components/ui/TabBar';
import { TopBar } from './components/ui/TopBar';
import { mockCurrentUser } from './data/mockData';
import { ContactsPage } from './pages/ContactsPage';
import { DiscoverPage } from './pages/DiscoverPage';
import { MePage } from './pages/MePage';
import { MessagesPage } from './pages/MessagesPage';

type TabKey = 'messages' | 'contacts' | 'discover' | 'me';

const tabs: TabDefinition<TabKey>[] = [
  { key: 'messages', label: '消息', icon: MessageCircle },
  { key: 'contacts', label: '联系人', icon: Contact },
  { key: 'discover', label: '发现', icon: Compass },
  { key: 'me', label: '我的', icon: UserRound },
];

type AppProps = {
  initialUser?: UserProfile;
  userApi?: UserApi;
};

function App({ initialUser = mockCurrentUser, userApi = defaultUserApi }: AppProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('messages');
  const [currentUser, setCurrentUser] = useState<UserProfile>(initialUser);
  const activeLabel = useMemo(() => tabs.find((tab) => tab.key === activeTab)?.label ?? '消息', [activeTab]);

  async function updateProfile(patch: UserProfilePatch) {
    const updatedUser = await userApi.patchCurrentUser(patch);
    setCurrentUser(updatedUser);
  }

  return (
    <main className="app-shell" aria-label="Agents IM 微信风格主框架">
      <section className="phone-frame">
        <TopBar title={activeLabel} />

        <section className="content-area">{renderPage(activeTab, currentUser, updateProfile)}</section>

        <TabBar tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />
      </section>
    </main>
  );
}

function renderPage(tab: TabKey, currentUser: UserProfile, onUpdateProfile: (patch: UserProfilePatch) => Promise<void>) {
  switch (tab) {
    case 'contacts':
      return <ContactsPage />;
    case 'discover':
      return <DiscoverPage />;
    case 'me':
      return <MePage profile={currentUser} onUpdateProfile={onUpdateProfile} />;
    case 'messages':
    default:
      return <MessagesPage />;
  }
}

export default App;
