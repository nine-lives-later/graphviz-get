FROM golang:1.18-alpine AS builder

#RUN apk add --no-cache git gcc musl-dev

WORKDIR /build
COPY . .

RUN go build ./...
#RUN go test -v ./...




FROM alpine:3 as graphviz-builder

ARG GRAPHVIZ_VERSION=4.0.0

WORKDIR /build

RUN apk add --no-cache make libtool automake autoconf pkgconfig g++ zlib-dev libpng-dev jpeg-dev groff ghostscript bison flex python3 expat-dev pango-dev libwebp-dev

RUN wget -q -O graphviz.tgz https://gitlab.com/graphviz/graphviz/-/archive/${GRAPHVIZ_VERSION}/graphviz-${GRAPHVIZ_VERSION}.tar.gz
RUN tar xzf graphviz.tgz --strip 1

# disable all warnings
ENV \
    CFLAGS=-w \
    CXXFLAGS=-w

RUN ./autogen.sh
RUN ./configure --prefix=/build/output
RUN make && make install




FROM alpine:3

# do not allow embedding of images
ENV SERVER_NAME="graphiviz-get"
ENV GV_FILE_PATH="/var/empty"

RUN mkdir -p -m 0555 /var/empty

RUN apk add --no-cache font-misc-misc ttf-freefont msttcorefonts-installer libltdl zlib libpng jpeg expat pango libwebp

RUN adduser -s /bin/sh -S -u 5741 graphviz;

COPY ./post_build_test.sh /usr/local/bin/
COPY --from=builder /build/graphviz-get /build/
COPY --from=graphviz-builder /build/output /usr/local/

USER graphviz

# test if it works
RUN /usr/local/bin/post_build_test.sh

EXPOSE 8080

CMD ["/build/graphviz-get"]
