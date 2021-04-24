package main

import (
	"fmt"

	"github.com/ipfs/go-cid"
)

func main() {
	c, _ := cid.Parse("baga6ea4seaqao7s73y24kcutaosvacpdjgfe5pw76ooefnyqw4ynr3d2y6x2mpq")
	fmt.Printf("%v\n", c.Bytes())
}
