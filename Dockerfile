FROM golang:1.24 as build

WORKDIR /app

COPY go.mod .

RUN go mod download

COPY . .

ENV GOOS linux
ENV GOARCH amd64
ENV CGO_ENABLED 1

RUN go build -a -installsuffix cgo -o app

FROM --platform=linux/amd64 debian

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates

RUN update-ca-certificates

WORKDIR /app

COPY --from=build /app/app /app

ENTRYPOINT [ "/app/app" ]