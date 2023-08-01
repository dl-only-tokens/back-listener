FROM golang:1.18-alpine as buildbase

RUN apk add git build-base

WORKDIR /go/src/github.com/dl-only-tokens/back-listener
COPY vendor .
COPY . .

RUN GOOS=linux go build  -o /usr/local/bin/back-listener /go/src/github.com/dl-only-tokens/back-listener


FROM alpine:3.9

COPY --from=buildbase /usr/local/bin/back-listener /usr/local/bin/back-listener
RUN apk add --no-cache ca-certificates

ENTRYPOINT ["back-listener"]
