package main

import (
	"fmt"
	"log"
	"path/filepath"
	
	"github.com/luxfi/database/badgerdb"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/core/rawdb"
)

func main() {
	path := filepath.Clean("/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb")
	db, err := badgerdb.New(path, nil, "", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	head := rawdb.ReadHeadHeaderHash(db)
	if head == (common.Hash{}) {
		log.Fatal("HeadHeader empty")
	}
	
	n := rawdb.ReadHeaderNumber(db, head)
	if n == nil {
		log.Fatal("H[head] missing")
	}
	
	hdr := rawdb.ReadHeader(db, head, *n)
	if hdr == nil {
		log.Fatal("Header(h+num+hash) missing")
	}
	
	canon := rawdb.ReadCanonicalHash(db, *n)
	if canon != head {
		log.Fatal("h[num]'n' != head")
	}
	
	g := rawdb.ReadCanonicalHash(db, 0)
	if g == (common.Hash{}) {
		log.Fatal("canonical[0] missing")
	}
	
	fmt.Printf("Tip %d %s OK; genesis %s OK\n", *n, head.Hex(), g.Hex())
}