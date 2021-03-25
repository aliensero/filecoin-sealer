## FFI

all: build
.PHONY: all

FFI_PATH:=extern/lotus/extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))
BUILD_DEPS:=.filecoin-install
CLEAN:=.update-module

$(FFI_DEPS): .filecoin-install ;

$(FFI_PATH): .update-module ;

.update-module:
	git submodule update --init --recursive
	@touch $@

.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

calibnet: GOFLAGS=-tags=calibnet
calibnet: build

ns-miner: $(BUILD_DEPS)
	rm -f ns-miner
	go build $(GOFLAGS) -o ns-miner ./cmd/miner
.PHONY: ns-miner
BINS+=ns-miner

ns-worker: $(BUILD_DEPS)
	rm -f ns-worker
	go build -o ns-worker ./cmd/worker
.PHONY: ns-worker
BINS+=ns-worker

build: ns-miner ns-worker

clean:
	rm -fr $(BINS) $(BUILD_DEPS) $(CLEAN)
	-$(MAKE) -C $(FFI_PATH) clean
.PHONY: clean