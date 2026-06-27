FROM golang:1.26-alpine

RUN apk add --no-cache \
    git \
    build-base \
    ffmpeg \
    imagemagick \
    libjpeg-turbo-utils \
    exiftool \
    optipng \
    libwebp-tools

RUN go install github.com/githubnemo/CompileDaemon@latest

WORKDIR /app

ENTRYPOINT ["CompileDaemon", "-log-prefix=false", "-directory=.", "-build=go build -o main ./cmd/app/main.go", "-command=./main serve"]