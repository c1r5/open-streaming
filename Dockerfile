# -------- Stage 1: Build --------
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./src ./src

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./src/cmd/server/main.go

# -------- Stage 2: Runtime --------
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 3000

CMD ["./server", "serve"]
