FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json .
ENV NODE_ENV=production
# Create unprivileged user
RUN addgroup -S appgroup && adduser -S -G appgroup -u 1000 appuser
USER appuser
EXPOSE 3000
CMD ["node", "build"]
