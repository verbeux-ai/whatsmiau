FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install gcc and SQLite dev libraries
RUN apk add build-base sqlite-dev gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o whatsmiau main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/whatsmiau /app/whatsmiau

RUN mkdir /app/data && chmod 777 -R /app/data

EXPOSE 8081

ENTRYPOINT ["./whatsmiau"]