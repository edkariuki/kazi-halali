# Kazi Halali web app
#
# Build from the web/ folder. The API address is fixed at BUILD time
# (Next.js inlines NEXT_PUBLIC_* variables into the browser bundle):
#   docker build -t kazi-halali-web --build-arg NEXT_PUBLIC_API_URL=https://your-api.example web
#
# Run:
#   docker run -p 3000:3000 kazi-halali-web

FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build
WORKDIR /app
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \
    NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN test -n "$NEXT_PUBLIC_API_URL" || (echo "Set --build-arg NEXT_PUBLIC_API_URL=<API URL>" && exit 1)
RUN npm run build

FROM node:22-alpine AS run
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
# "standalone" output contains only what the server needs.
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
EXPOSE 3000
USER node
CMD ["node", "server.js"]