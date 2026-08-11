FROM node:22.12-alpine AS builder

WORKDIR /app
RUN npm install --global pnpm@10.15.1
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.base.json ./
COPY openapi ./openapi
COPY web ./web
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile && pnpm build

FROM nginx:1.29-alpine
COPY web/nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /app/resource/web/admin /usr/share/nginx/html/admin
COPY --from=builder /app/resource/web/status /usr/share/nginx/html/status
COPY --from=builder /app/resource/web/api-docs /usr/share/nginx/html/docs/api
ENV DASHBOARD_UPSTREAM=http://santaizi-dashboard:80
EXPOSE 80
