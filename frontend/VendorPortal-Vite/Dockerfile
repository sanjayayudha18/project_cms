# Stage 1: Install dependencies and build
FROM node:20-alpine AS builder
RUN npm install -g pnpm@9
WORKDIR /app
COPY pnpm-lock.yaml package.json ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

# Stage 2: Serve with nginx
FROM nginx:alpine
LABEL org.opencontainers.image.title="vendorportal-vite-fe"
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
