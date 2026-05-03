# AI Reply V1 Design

Status: Draft / accepted for V1 implementation

## Background

`agents_im` is adding AI-assisted replies to IM conversations. The first version must be safe, explicit, and cheap enough to use in long conversations. It must not auto-send messages or send the entire conversation history to a model.

## Goals

- Per-user, per-conversation AI reply settings.
- Default disabled for every conversation.
- V1 mode is manual `suggest_only`: the user clicks an AI reply button to generate a draft.
- The draft is editable and is sent only when the user explicitly sends it through the existing message-send path.
- Context is bounded: recent messages plus optional summary, never unbounded full history.
- Missing/invalid model configuration fails visibly; production paths must not return fake AI replies.

## Non-goals

- No automatic AI sending in V1.
- No always-on bot behavior.
- No group-chat noise from unsolicited AI replies.
- No full prompt/prompt-history persistence unless a later security/product design approves it.
- No fake production provider. Test fake providers are allowed only inside tests.

## Setting Scope

The setting belongs to the current user in a conversation, not to the conversation globally.

```text
(owner_account_id, conversation_id)
```

Reason: Alice may want AI help in a chat while Bob does not. One user enabling AI must not change another user's experience.

Suggested fields:

```text
conversation_ai_settings
- owner_account_id
- conversation_id
- enabled bool default false
- mode smallint or text: disabled/suggest_only; auto_reply reserved for future
- max_recent_messages default 30
- summary_enabled bool default true or false depending implementation maturity
- created_at
- updated_at
```

If the implementation starts with in-memory settings for tests, the interface and API should still reflect this persistent shape so PostgreSQL can be added without contract churn.

## User Experience

V1 flow:

1. User opens a conversation.
2. User may enable AI reply suggestions for that conversation.
3. User clicks `AI 回复` / `AI 帮我回`.
4. Backend generates a draft from bounded context.
5. Frontend displays an editable draft or fills the composer.
6. User edits if needed.
7. User sends via existing `POST /messages`.

Important: generating a draft must not create a `messages` row.

## API Contract

Suggested endpoints, following existing REST/go-zero project conventions:

```text
GET /conversations/:conversation_id/ai-settings
PUT /conversations/:conversation_id/ai-settings
POST /conversations/:conversation_id/ai-replies/generate
```

Settings response:

```json
{
  "conversation_id": "...",
  "enabled": false,
  "mode": "disabled",
  "max_recent_messages": 30,
  "summary_enabled": true
}
```

Generate response:

```json
{
  "draft": "...",
  "context_until_seq": 123,
  "recent_message_count": 30,
  "summary_used": false
}
```

Failure examples:

- disabled setting: return a visible domain error unless request explicitly says one-shot generation is allowed.
- missing model config: visible 5xx/typed error, not fake text.
- no access to conversation: 403/404 following existing access-control conventions.

## Context Management

Do not send full conversation history.

V1 prompt input should be composed from:

1. System instruction: the model is drafting a reply for the current user, not acting as an autonomous participant.
2. Optional conversation summary.
3. Bounded recent messages, default about 30 messages.
4. Current target/latest message metadata.

Suggested prompt constraints:

```text
- Reply as the current user.
- Do not invent facts.
- Be concise and natural.
- Output only the draft message text.
```

### Recent Window

Default:

```text
max_recent_messages = 30
```

Implementation should also enforce a rough prompt-size cap if token estimation exists. If not, cap by message count and truncate extremely long message bodies defensively.

### Rolling Summary Placeholder

V1 may create interfaces/schema placeholders for rolling summaries but does not have to implement asynchronous summary refresh.

Future shape:

```text
conversation_ai_summaries
- conversation_id
- owner_account_id nullable if user-specific summaries are needed
- summary_text
- covered_until_seq
- version
- updated_at
```

AI generation can use:

```text
summary where covered_until_seq < first_recent_seq
+ recent messages after covered_until_seq
```

If summary is unavailable, generate from recent messages only and return `summary_used=false`.

## Provider Boundary

Add a small provider interface, for example:

```go
type AIReplyGenerator interface {
    GenerateReply(ctx context.Context, req GenerateReplyRequest) (GenerateReplyResult, error)
}
```

Production provider should be config-driven and fail closed when not configured. Tests may inject a fake provider.

Do not log full prompt or tokens. Logs should include request IDs and error class, not sensitive message content.

## Message Semantics

- AI draft generation is not a message.
- If/when user sends the draft, it is a normal human-origin message because the user chose and sent it.
- Future auto-reply mode, if implemented, may use `message_origin=ai` and must go through Message Service.

## Frontend Contract

Messages UI should expose:

- Conversation-level AI reply status.
- Toggle or setting control.
- `AI 回复` generate button when appropriate.
- Loading state while generating.
- Visible error if generation fails.
- Editable draft composer integration.

Tests should use a fake API client/provider; production UI must not silently fake success.

## Verification

Recommended checks:

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
for f in api/*.api; do goctl api validate -api "$f"; done
go test ./internal/logic ./internal/handler ./internal/repository ./tests
npm --prefix web run test:run -- src/features/messages/MessagesPage.test.tsx --reporter=verbose
npm --prefix web run build
bash scripts/verify-static.sh
git diff --check
```

If model/provider credentials are not configured, live generation should fail visibly and tests should cover the failure path.
