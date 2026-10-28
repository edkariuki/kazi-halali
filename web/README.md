# Kazi Halali — web

Next.js + Tailwind frontend. See the [root README](../README.md) for the full setup.

```bash
cp .env.example .env.local   # NEXT_PUBLIC_API_URL, defaults to http://localhost:8080
npm install
npm run dev
```

- `/` — paste an ad or upload a screenshot, get a verdict
- `/c/<id>` — shareable result page

Fonts are self-hosted from `@fontsource` (no request to Google Fonts), which keeps the
page fast on slow mobile connections.
