# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY server/package*.json ./server/

# Install ALL dependencies (including devDependencies like typescript)
RUN cd server && npm install

# Copy server code
COPY server/ ./server/

# Copy .env for path reference
COPY .env ./

# Build the server using npx to ensure tsc is found
RUN cd server && npx tsc

# Production stage
FROM node:20-alpine

# Install curl for healthcheck
RUN apk add --no-cache curl

WORKDIR /app

# Copy built files and production dependencies
COPY --from=builder /app/server/dist ./server/dist
COPY --from=builder /app/server/package*.json ./server/
COPY --from=builder /app/.env ./

# Install only production dependencies
RUN cd server && npm install --omit=dev

# Expose port
EXPOSE 3000

# Start command
CMD ["node", "server/dist/index.js"]
