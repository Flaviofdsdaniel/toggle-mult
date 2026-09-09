FROM golang:1.21-alpine

WORKDIR /app

COPY auth-service/go.mod ./
RUN go mod download

COPY auth-service/*go ./
RUN go mod tidy
COPY auth-service/ .

CMD ["go", "run", "."]