FROM golang:1.25-alpine

WORKDIR /app

COPY auth-service/go.mod auth-service/go.sum ./

RUN go mod download

COPY auth-service/ .

RUN go mod tidy

CMD ["go", "run", "."]