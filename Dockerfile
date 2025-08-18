FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install gcc and SQLite dev libraries
RUN apk add build-base sqlite-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o whatsmiau main.go

FROM alpine:latest

WORKDIR /app

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/whatsmiau /app/whatsmiau

RUN mkdir data && chown -R appuser:appgroup /app/data

# Switch to the non-root user
USER appuser

EXPOSE 8081

CMD ["./whatsmiau"]