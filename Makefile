GO_SRCS := $(wildcard *.go config/*.go)

.PHONY: front contextdict

contextdict: $(GO_SRCS) front
	go build .

front:
	$(MAKE) -C ./frontend

# 根据 charts 中 values.yaml 生成 config.yaml, 方便开发
config.yaml: ./charts/contextdict/values.yaml
	helm template contextdict ./charts/contextdict | yq 'select(.data."config.yaml" != null) | .data."config.yaml"' > config.yaml

run: contextdict config.yaml
	./contextdict
