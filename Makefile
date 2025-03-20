GO_SRCS := $(wildcard *.go config/*.go)

.PHONY: front contextdict

contextdict: $(GO_SRCS) front
	go build .

front:
	$(MAKE) -C ./frontend

run: contextdict
	./contextdict
