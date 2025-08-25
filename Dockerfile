FROM golang:alpine

WORKDIR /usr/src/app

RUN apk add --no-cache vips-dev gcc musl-dev

COPY --link go.mod go.sum ./
RUN go mod download

COPY --link . .
# RUN go install -ldflags="-s -w" -v ./...

# FROM alpine

# COPY --from=0 /go/bin/imly /usr/local/bin

# RUN set -eux; \
# 	runDeps="$( \
# 		scanelf --needed --nobanner --format '%n#p' --recursive /usr/local \
# 			| tr ',' '\n' \
# 			| sort -u \
# 			| awk 'system("[ -e /usr/local/lib/" $1 " ]") == 0 { next } { print "so:" $1 }' \
# 	)"; \
# 	apk add --no-cache $runDeps

# ENTRYPOINT ["/usr/local/bin/imly"]
