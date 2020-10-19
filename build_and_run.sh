#!/bin/sh

set -e

docker build -t graphvizget387465 .

docker run -it -p 8080:8080 graphvizget387465
