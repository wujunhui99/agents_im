import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { FeedbackPage } from './FeedbackPage';

describe('FeedbackPage', () => {
  it('submits polished user feedback with formal categories and selected image context', async () => {
    const user = userEvent.setup();
    const onSubmitFeedback = vi.fn(async () => undefined);

    render(<FeedbackPage onSubmitFeedback={onSubmitFeedback} />);

    const feedbackForm = screen.getByRole('region', { name: '意见反馈表单' });
    expect(within(feedbackForm).getByRole('radio', { name: /问题反馈/ })).toBeChecked();
    expect(within(feedbackForm).getByRole('radio', { name: /体验建议/ })).toBeInTheDocument();
    expect(within(feedbackForm).getByRole('radio', { name: /功能建议/ })).toBeInTheDocument();
    expect(within(feedbackForm).getByRole('radio', { name: /其他反馈/ })).toBeInTheDocument();
    expect(within(feedbackForm).queryByText('Bug')).not.toBeInTheDocument();
    expect(within(feedbackForm).queryByText('体验不好')).not.toBeInTheDocument();

    await user.click(within(feedbackForm).getByRole('radio', { name: /体验建议/ }));
    await user.type(within(feedbackForm).getByLabelText('反馈标题'), '消息发送入口不明显');
    await user.type(within(feedbackForm).getByLabelText('详细说明'), '输入区域层级不清晰，希望更容易找到发送按钮');
    await user.type(within(feedbackForm).getByLabelText('联系方式（选填）'), 'alice@example.com');
    await user.upload(
      within(feedbackForm).getByLabelText('添加截图或图片'),
      new File([new Uint8Array(1024)], 'feedback-screen.png', { type: 'image/png' }),
    );

    expect(within(feedbackForm).getByText('feedback-screen.png')).toBeInTheDocument();
    expect(within(feedbackForm).getByText('图片将在后续版本上传到反馈工单')).toBeInTheDocument();

    await user.click(within(feedbackForm).getByRole('button', { name: '提交反馈' }));

    await waitFor(() =>
      expect(onSubmitFeedback).toHaveBeenCalledWith({
        category: 'poor_experience',
        title: '消息发送入口不明显',
        content: '输入区域层级不清晰，希望更容易找到发送按钮',
        contact: 'alice@example.com',
        pageUrl: expect.any(String),
        userAgent: expect.any(String),
        clientMeta: {
          attachmentFileNames: ['feedback-screen.png'],
          attachmentCount: 1,
          attachmentUploadStatus: 'local_selection_only',
        },
      }),
    );
    expect(await screen.findByRole('status')).toHaveTextContent('反馈已提交');
  });
});
