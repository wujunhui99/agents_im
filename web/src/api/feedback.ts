import { createApiClient, type ApiClient } from './client';

export type SubmitFeedbackRequest = {
  category: 'bug' | 'poor_experience' | 'feature_request' | 'other' | string;
  title: string;
  content: string;
  contact?: string;
  pageUrl?: string;
  userAgent?: string;
  clientMeta?: Record<string, unknown>;
};

export type SubmitFeedbackResponse = {
  feedbackId: string;
  status: string;
};

export type FeedbackApi = {
  submitFeedback: (request: SubmitFeedbackRequest) => Promise<SubmitFeedbackResponse>;
};

export function createFeedbackApi(api: ApiClient = createApiClient()): FeedbackApi {
  return {
    submitFeedback(request) {
      return api.post<SubmitFeedbackResponse>('/api/feedback', request);
    },
  };
}
