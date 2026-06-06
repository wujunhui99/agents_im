# React + Vite Patterns

- Keep page-level behavior in small React components.
- Prefer pure data arrays for mock list content.
- Use Testing Library queries by role/text instead of implementation details.
- API clients should be thin wrappers around `fetch` with typed request/response shapes.
- Store bearer tokens in an auth state abstraction for MVP; do not hardcode secrets.
- WebSocket wrappers should expose connect/send/close and translate server snake_case ACKs into typed frontend objects.
