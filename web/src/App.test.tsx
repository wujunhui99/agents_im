import { render, screen, waitFor, within } from '@testing-library/react';
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

  it('shows the seeded conversation list on the messages tab', () => {
    render(<App />);

    const list = screen.getByRole('list', { name: '消息列表' });
    expect(within(list).getByRole('button', { name: /产品讨论群/ })).toBeInTheDocument();
    expect(within(list).getByRole('button', { name: /junhui/ })).toBeInTheDocument();
    expect(screen.getByText('后端 MVP 已发布，开始搭前端主框架。')).toBeInTheDocument();
  });

  it('opens a conversation and returns to the list on mobile navigation', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: /产品讨论群/ }));

    expect(screen.getByRole('heading', { name: '产品讨论群' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '输入消息' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: '返回消息列表' }));

    expect(screen.getByRole('list', { name: '消息列表' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /产品讨论群/ })).toBeInTheDocument();
  });

  it('appends a sending message from the composer before marking it sent', async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: /junhui/ }));
    await user.type(screen.getByRole('textbox', { name: '输入消息' }), '这是测试消息');
    await user.click(screen.getByRole('button', { name: '发送' }));

    expect(screen.getByText('这是测试消息')).toBeInTheDocument();
    expect(screen.getByText('发送中')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByText('已发送')).toBeInTheDocument());
  });
});
