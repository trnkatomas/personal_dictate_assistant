FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# adapter-node bundles the SvelteKit runtime but not external npm deps (diff, diff2html, …).
# Install only production deps in the final image.
FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/build ./build
COPY package*.json ./
RUN npm ci --omit=dev

# Allow large audio uploads (64 MB). adapter-node reads BODY_SIZE_LIMIT at startup.
ENV BODY_SIZE_LIMIT=67108864

EXPOSE 3000
CMD ["node", "build"]
#ENV NODE_ENV=production
#CMD ["node", "app.js"]
#CMD /bin/bash
