# Stage 1: Build
FROM node:22-alpine AS builder
RUN npm install -g pnpm@11.20.0
WORKDIR /app
COPY pnpm-lock.yaml package.json pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm tsc -b tsconfig.app.json && pnpm vite build

# Stage 2: Serve
FROM nginx:alpine
LABEL org.opencontainers.image.title="userportal-vite-fe"
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
