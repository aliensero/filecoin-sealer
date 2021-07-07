package chain

import "testing"

func TestDaemon(t *testing.T) {
	Daemon("https://filestar.info/rpc/v0", "127.0.0.1:1080")
}
