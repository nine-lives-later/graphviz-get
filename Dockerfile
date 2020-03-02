FROM golang:1.13-alpine AS builder

#RUN apk add --no-cache git gcc musl-dev

WORKDIR /build
COPY . .

RUN go build ./...
#RUN go test -v ./...





FROM alpine:3.11

# do not allow embedding of images
ENV SERVER_NAME="graphiviz-get"
ENV GV_FILE_PATH="/var/empty"

RUN mkdir -p /var/empty && chmod 0400 /var/empty

RUN apk add --no-cache graphviz font-misc-misc

COPY --from=builder /build/graphviz-get /build/

EXPOSE 8080

CMD ["/build/graphviz-get"]
