FROM golang:1.25-alpine

WORKDIR /app

COPY evaluation-service/go.mod evaluation-service/go.sum ./
RUN go mod download

COPY evaluation-service/*go ./
RUN go mod tidy
COPY evaluation-service/ .

CMD ["go", "run", "."]
