#!/bin/sh

set -e

DOCKER_BUILDKIT=1 docker build -t graphvizget387465 .

DEBUG=0

docker run -it -p 8080:8080 -e "DEBUG=$DEBUG" graphvizget387465
