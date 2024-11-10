FROM golang:latest as builder
WORKDIR /app
COPY . .
WORKDIR /app/cmd/server
RUN GOOS=linux CGO_ENABLED=0 go build -ldflags="-w -s" -o app

FROM scratch
WORKDIR /app
COPY --from=builder /app/cmd/server/app .
COPY cmd/server/config.yaml .
ENTRYPOINT [ "./app" ]
