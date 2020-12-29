GO := go
SRCS := $(shell find . -type f -iname "*.go")
OUTPUT := $(shell mkdir -p bin; echo bin)
BIN := remote_render

$(OUTPUT)/$(BIN): $(SRCS)
	go build -o $(OUTPUT)/$(BIN) main.go

.PHONY: clean
clean:
	rm -fr $(OUTPUT)

.PHONY: run
run: $(OUTPUT)/$(BIN)
	./$(OUTPUT)/$(BIN) -d /tmp/ddd -o /tmp/output -u doug -a 39.98.122.34 -s 4 -b 30
