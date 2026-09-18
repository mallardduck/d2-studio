// Shown on first launch so the edit -> compile -> preview loop works immediately.
export const SAMPLE = `# D2 Studio sample: edit me, the preview updates as you type.
direction: right

user: User {shape: person}

web: Web App {
  ui: React UI
  api: Go Service
  ui -> api: calls
}

db: Postgres {shape: cylinder}
cache: Redis {shape: cylinder}

user -> web.ui: browses
web.api -> db: reads / writes
web.api -> cache: caches
`;
