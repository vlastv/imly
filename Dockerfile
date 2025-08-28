FROM golang:alpine AS build

WORKDIR /usr/src/app

RUN apk add --no-cache vips-dev gcc musl-dev --repository=https://dl-cdn.alpinelinux.org/alpine/edge/community

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go install -ldflags="-s -w" -v ./...

FROM alpine:edge

RUN apk add --no-cache vips

COPY --from=0 /go/bin/imly /usr/local/bin

ENTRYPOINT ["/usr/local/bin/imly"]
