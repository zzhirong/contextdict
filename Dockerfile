# Build frontend
FROM node:23-alpine as frontend-builder
COPY ./frontend /frontend
WORKDIR /frontend
RUN npm install
RUN npm run build

# Build backend
FROM golang:1.23-alpine as backend-builder
run apk add build-base
WORKDIR /app
ARG APP_NAME
COPY . ./
COPY --from=frontend-builder /frontend/dist ./frontend/dist
RUN go mod download
RUN CGO_ENABLED=1 go build -o contextdict

FROM alpine:latest
ARG APP_NAME
WORKDIR /app
COPY --from=backend-builder /app/contextdict ./
EXPOSE 8085
CMD ["./contextdict"]
