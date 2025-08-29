FROM golang:alpine AS build

WORKDIR /usr/src/app

RUN apk add --no-cache vips-dev gcc musl-dev jemalloc-dev --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community

COPY go.mod go.sum ./
RUN go mod download

ENV CGO_CFLAGS="-fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free"
ENV CGO_LDFLAGS="-ljemalloc"

COPY . .
RUN go install -ldflags="-s -w" -v ./...

FROM alpine:edge

RUN apk add --no-cache vips jemalloc

COPY --from=0 /go/bin/imly /usr/local/bin

ENTRYPOINT ["/usr/local/bin/imly"]
