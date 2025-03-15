# Define app name at the beginning
ARG APP_NAME=contextdict

# Build frontend
FROM node:23-alpine as frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build backend
FROM golang:1.23-alpine as backend-builder
WORKDIR /app
ARG APP_NAME
RUN go mod download
RUN go build -o ${APP_NAME}

FROM alpine:latest
ARG APP_NAME
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
COPY --from=backend-builder /app/${APP_NAME} ./
COPY config.yaml ./
VOLUME "/app/cache"
EXPOSE 8085
CMD ["./${APP_NAME}"]
