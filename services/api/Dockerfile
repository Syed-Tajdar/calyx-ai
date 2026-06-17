# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

COPY services/api/package.json ./
RUN npm install

COPY services/api/ ./
COPY shared/providers/catalog.json ./catalog.json
RUN npm run build

# Runtime stage
FROM node:20-alpine

WORKDIR /app

COPY services/api/package.json ./
RUN npm install --omit=dev

COPY --from=builder /app/dist ./dist
COPY --from=builder /app/catalog.json ./catalog.json

ENV PROVIDER_CATALOG_PATH=/app/catalog.json

EXPOSE 3000

USER node

CMD ["node", "dist/main.js"]
