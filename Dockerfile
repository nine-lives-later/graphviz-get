FROM golang:1.15-alpine AS builder

#RUN apk add --no-cache git gcc musl-dev

WORKDIR /build
COPY . .

RUN go build ./...
#RUN go test -v ./...




FROM alpine:3.11 as graphviz-builder

ARG GRAPHVIZ_VERSION=2.44.1

WORKDIR /build

RUN apk add --no-cache make libtool automake autoconf pkgconfig g++ zlib-dev libpng-dev jpeg-dev groff ghostscript bison flex

RUN wget -q -O graphviz.tgz https://gitlab.com/graphviz/graphviz/-/archive/${GRAPHVIZ_VERSION}/graphviz-${GRAPHVIZ_VERSION}.tar.gz
RUN tar xzf graphviz.tgz --strip 1
RUN ./autogen.sh
RUN ./configure --prefix=/build/output
RUN make && make install




FROM alpine:3.11

# do not allow embedding of images
ENV SERVER_NAME="graphiviz-get"
ENV GV_FILE_PATH="/var/empty"

RUN mkdir -p /var/empty && chmod 0400 /var/empty

RUN apk add --no-cache font-misc-misc libltdl

COPY --from=builder /build/graphviz-get /build/
COPY --from=graphviz-builder /build/output /usr/local/

# test if it works
RUN /usr/local/bin/dot -V

EXPOSE 8080

CMD ["/build/graphviz-get"]
