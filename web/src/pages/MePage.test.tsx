import { render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { UserProfile } from '../api/user';
import { MePage } from './MePage';

const profile: UserProfile = {
  user_id: '1001',
  identifier: 'alice_001',
  display_name: 'Alice Chen',
  name: 'Alice Chen',
  gender: 'female',
  age: 30,
  region: 'Shanghai',
  account_type: 'user',
};

describe('MePage', () => {
  it('shows profile labels without exposing the internal user id', () => {
    render(<MePage profile={profile} onUpdateProfile={vi.fn()} />);

    const details = screen.getByRole('region', { name: '个人资料详情' });
    expect(within(details).queryByText('user_id')).not.toBeInTheDocument();
    expect(within(details).queryByText('1001')).not.toBeInTheDocument();
    expect(within(details).getByText('alice_001')).toBeInTheDocument();
    expect(within(details).getByText('Alice Chen')).toBeInTheDocument();
    expect(within(details).getByText('用户')).toBeInTheDocument();
  });
});
