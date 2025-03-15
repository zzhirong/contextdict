GO_SRCS := $(wildcard *.go config/*.go)

.PHONY: front ContextDict

run: front ContextDict
	./ContextDict

ContextDict: $(GO_SRCS)
	go build .

front:
	$(MAKE) -C ./frontend
