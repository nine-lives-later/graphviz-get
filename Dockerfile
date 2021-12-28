FROM golang:1.17-alpine AS builder

#RUN apk add --no-cache git gcc musl-dev

WORKDIR /build
COPY . .

RUN go build ./...
#RUN go test -v ./...




FROM alpine:3 as graphviz-builder

ARG GRAPHVIZ_VERSION=2.50.0

WORKDIR /build

RUN apk add --no-cache make libtool automake autoconf pkgconfig g++ zlib-dev libpng-dev jpeg-dev groff ghostscript bison flex python3 expat-dev

RUN wget -q -O graphviz.tgz https://gitlab.com/graphviz/graphviz/-/archive/${GRAPHVIZ_VERSION}/graphviz-${GRAPHVIZ_VERSION}.tar.gz
RUN tar xzf graphviz.tgz --strip 1
RUN ./autogen.sh
RUN mkdir -p /build/output/include   # prevent missing dir warning
RUN ./configure --prefix=/build/output --with-expat=yes
RUN make && make install




FROM alpine:3

# do not allow embedding of images
ENV SERVER_NAME="graphiviz-get"
ENV GV_FILE_PATH="/var/empty"

RUN mkdir -p -m 0555 /var/empty

RUN apk add --no-cache font-misc-misc libltdl

RUN adduser -s /bin/sh -S -u 5741 graphviz;

COPY --from=builder /build/graphviz-get /build/
COPY --from=graphviz-builder /build/output /usr/local/

USER graphviz

# test if it works
RUN /usr/local/bin/dot -V

EXPOSE 8080

CMD ["/build/graphviz-get"]
