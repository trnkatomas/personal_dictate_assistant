FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# adapter-node bundles everything; the production image only needs the build output.
FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/build ./build

# Allow large audio uploads (64 MB). adapter-node reads BODY_SIZE_LIMIT at startup.
ENV BODY_SIZE_LIMIT=67108864

EXPOSE 3000
CMD ["node", "build"]
