VUE_FILES := $(shell find . -type f -name '*.vue')
./dist/index.html: $(VUE_FILES)
	pnpm install && pnpm run build
