### How to generate tile cache

First, download the file:
> curl https://openslide.cs.cmu.edu/download/openslide-testdata/Aperio/CMU-1.svs -o CMU-1.svs

Next generate cache:
> go run examples/generate-cache/main.go --file=CMU-1.svs