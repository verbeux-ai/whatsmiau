FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o whatsmiau main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/whatsmiau .

EXPOSE 8080

CMD ["./whatsmiau"]
