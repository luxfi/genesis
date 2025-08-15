package main

import (
    "encoding/hex"
    "fmt"
    "path/filepath"
    "time"
    
    "github.com/luxfi/database/badgerdb"
    "golang.org/x/crypto/sha3"
)

func keccak256(data []byte) []byte {
    hasher := sha3.NewLegacyKeccak256()
    hasher.Write(data)
    return hasher.Sum(nil)
}

func main() {
    dbPath := "/Users/z/.luxd/network-96369/chains/EWi9aPkGe6EfJ3SobCAmSUXRPLa4brF3cThwPwmHTrD1y13jy/ethdb"
    
    fmt.Println("Adding hash-scheme keys to database...")
    db, err := badgerdb.New(filepath.Clean(dbPath), nil, "", nil)
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // Scan path-scheme nodes and add hash-scheme versions
    added := 0
    startTime := time.Now()
    
    for prefix := byte(0x00); prefix <= 0x09; prefix++ {
        iter := db.NewIterator()
        
        batch := db.NewBatch()
        batchSize := 0
        
        for iter.Next() {
            key := iter.Key()
            value := iter.Value()
            
            // Only process path-scheme nodes
            if len(key) > 0 && key[0] == prefix && len(value) > 0 {
                // Create hash-scheme key
                hashKey := keccak256(value)
                
                // Add if not exists
                if _, err := db.Get(hashKey); err != nil {
                    batch.Put(hashKey, value)
                    batchSize += len(hashKey) + len(value)
                    added++
                    
                    if batchSize > 50*1024 { // 50KB batches
                        batch.Write()
                        batch = db.NewBatch()
                        batchSize = 0
                    }
                }
            }
            
            if added%10000 == 0 && added > 0 {
                fmt.Printf("Added %d hash keys...\n", added)
            }
        }
        
        if batchSize > 0 {
            batch.Write()
        }
        iter.Release()
    }
    
    // Add state root explicitly
    stateRoot, _ := hex.DecodeString("aedd8be7a060b082b0cb3195d0b5ba017c058468851ed93dd07eca274de000c2")
    
    // Find the root node in path-scheme
    iter := db.NewIterator()
    for iter.Next() {
        value := iter.Value()
        if len(value) > 32 {
            hash := keccak256(value)
            if string(hash) == string(stateRoot) {
                fmt.Printf("Found state root node! Adding hash key...\n")
                db.Put(stateRoot, value)
                break
            }
        }
    }
    iter.Release()
    
    elapsed := time.Since(startTime)
    fmt.Printf("Added %d hash-scheme keys in %s\n", added, elapsed.Round(time.Second))
}
