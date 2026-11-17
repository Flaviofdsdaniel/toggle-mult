FROM golang:1.25-alpine AS build

WORKDIR /src
COPY auto-recuperacao-microsservicos/go.mod ./go.mod
COPY auto-recuperacao-microsservicos/main.go ./main.go
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auto-recuperacao .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auto-recuperacao /auto-recuperacao
EXPOSE 8080
ENTRYPOINT ["/auto-recuperacao"]
