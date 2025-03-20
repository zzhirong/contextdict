# Define app name at the beginning
ARG APP_NAME=contextdict

# Build frontend
FROM node:23-alpine as frontend-builder
COPY ./frontend /frontend
WORKDIR /frontend
RUN npm install
RUN npm run build

# Build backend
FROM golang:1.23-alpine as backend-builder
WORKDIR /app
ARG APP_NAME
COPY . ./
COPY --from=frontend-builder /frontend/dist ./frontend/dist
RUN go mod download
RUN go build -o ${APP_NAME}

FROM alpine:latest
ARG APP_NAME
WORKDIR /app
COPY --from=backend-builder /app/${APP_NAME} ./
VOLUME "/app/data"
EXPOSE 8085
CMD ["./${APP_NAME}"]
