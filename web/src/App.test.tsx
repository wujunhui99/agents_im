import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import App from './App';

describe('WeChat-inspired app shell', () => {
  it('renders the four primary tabs', () => {
    render(<App />);

    expect(screen.getByRole('tab', { name: /消息/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /联系人/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /发现/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /我的/i })).toBeInTheDocument();
  });

  it('defaults to the messages page and switches pages from the bottom navigation', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByRole('heading', { name: '消息' })).toBeInTheDocument();
    expect(screen.getByText('产品讨论群')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    expect(screen.getByRole('heading', { name: '联系人' })).toBeInTheDocument();
    expect(screen.getByText('新的朋友')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /发现/i }));
    expect(screen.getByRole('heading', { name: '发现' })).toBeInTheDocument();
    expect(screen.getByText('朋友圈')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: /我的/i }));
    expect(screen.getByRole('heading', { name: '我的' })).toBeInTheDocument();
    expect(screen.getByText(/alice_001/)).toBeInTheDocument();
  });

  it('shows contact entry points and groups friends by initial', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));

    expect(screen.getByRole('button', { name: /新的朋友/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /群聊/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /标签/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /公众号/i })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'A' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'B' })).toBeInTheDocument();
    expect(screen.getByText('Alice Chen')).toBeInTheDocument();
    expect(screen.getByText('Bob Lin')).toBeInTheDocument();
  });

  it('searches users by unique identifier', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));

    const searchRegion = screen.getByRole('region', { name: '账号搜索' });
    expect(screen.getByRole('status')).toHaveTextContent('找到 Bob Lin');
    expect(within(searchRegion).getByText('bob_002')).toBeInTheDocument();
  });

  it('exposes an add friend action from identifier search results', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.type(screen.getByLabelText('按 identifier 搜索用户'), 'bob_002');
    await user.click(screen.getByRole('button', { name: '搜索用户' }));
    await user.click(screen.getByRole('button', { name: '添加好友 bob_002' }));

    expect(screen.getByRole('status')).toHaveTextContent('已发送添加请求');
  });

  it('opens group details and renders the member list', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('tab', { name: /联系人/i }));
    await user.click(screen.getByRole('button', { name: /群聊/i }));
    await user.click(screen.getByRole('button', { name: /查看 Frontend Demo/i }));

    expect(screen.getByRole('heading', { name: 'Frontend Demo' })).toBeInTheDocument();
    expect(screen.getByRole('list', { name: '群成员列表' })).toBeInTheDocument();
    expect(screen.getByText('Alice Chen')).toBeInTheDocument();
    expect(screen.getByText('Bob Lin')).toBeInTheDocument();
  });
});
