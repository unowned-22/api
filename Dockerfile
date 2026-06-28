FROM golang:1.26-alpine

RUN apk add --no-cache \
    git \
    build-base \
    ffmpeg \
    vips-dev

RUN go install github.com/githubnemo/CompileDaemon@latest

WORKDIR /app

CMD ["CompileDaemon", "--build=go build -o app ./cmd/app", "--command=./app serve"]