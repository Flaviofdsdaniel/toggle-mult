FROM golang:1.21-alpine AS evaluation-service

WORKDIR /app

COPY evaluation-service/go.mod evaluation-service/go.sum ./
RUN go mod download

COPY evaluation-service/*go ./
RUN go mod tidy
COPY evaluation-service/ .

CMD ["go", "run", "."]
