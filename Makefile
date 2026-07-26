BINARY := bleatcode

.PHONY: build clean

build:
	go build -o $(BINARY) ./cmd/bleatcode

clean:
	rm -f $(BINARY)
