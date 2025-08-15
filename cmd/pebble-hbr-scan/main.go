package main

import (
	"fmt"
	"os"
	
	"github.com/cockroachdb/pebble"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: scan <pebble_path>")
	}
	
	db, err := pebble.Open(os.Args[1], &pebble.Options{ReadOnly: true})
	if err != nil {
		panic(err)
	}
	defer db.Close()
	
	it, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		panic(err)
	}
	defer it.Close()
	
	foundH, foundB, foundR, foundMap := 0, 0, 0, 0
	
	for it.First(); it.Valid(); it.Next() {
		k := it.Key()
		if len(k) < 33 {
			continue
		}
		
		// SubnetEVM layout typically: ns(32) + tag + ...
		tag := k[32]
		switch tag {
		case 'h':
			if len(k) >= 41 {
				foundH++ // header: ns + 'h' + num(8) + hash(32)
			}
		case 'b':
			if len(k) >= 41 {
				foundB++ // bodies
			}
		case 'r':
			if len(k) >= 41 {
				foundR++ // receipts
			}
		case 'H':
			if len(k) >= 33 {
				foundMap++ // hash->number: ns + 'H' + hash(32)
			}
		}
		
		if foundH > 50 && foundB > 50 && foundR > 50 && foundMap > 50 {
			break
		}
	}
	
	fmt.Printf("headers:%d bodies:%d receipts:%d H->num:%d\n", foundH, foundB, foundR, foundMap)
}