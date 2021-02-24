module gitlab.ns/lotus-worker

go 1.14


replace (
    github.com/filecoin-project/filecoin-ffi => ./extern/lotus/extern/filecoin-ffi
    github.com/supranational/blst => ./extern/lotus/extern/blst
    github.com/filecoin-project/lotus => ./extern/lotus
)