#!/bin/sh

set -e

echo "Testing generic call..."
/usr/local/bin/dot -V > /dev/null

echo "Testing SVG..."
/usr/local/bin/dot -Tsvg <<- EOM > /dev/null
digraph G {
    A;
    B;
    A -> B;
}
EOM

echo "Testing HTML-like label..."
/usr/local/bin/dot -Tsvg <<- EOM > /dev/null
digraph G {
    A;
    B [label=<<TABLE><TR><TD>B</TD></TR></TABLE>>];
    A -> B;
}
EOM

echo "Testing PNG..."
/usr/local/bin/dot -Tpng <<- EOM > /dev/null
digraph G {
    A;
    B;
    A -> B;
}
EOM

echo "Testing PDF..."
/usr/local/bin/dot -Tpdf <<- EOM > /dev/null
digraph G {
    A;
    B;
    A -> B;
}
EOM

echo "Testing WEBP..."
/usr/local/bin/dot -Twebp <<- EOM > /dev/null
digraph G {
    A;
    B;
    A -> B;
}
EOM

echo "Testing done."
