package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "fmt"
    "math/big"
    "os"
    "time"
    
    "github.com/luxfi/node/staking"
)

func main() {
    // Generate a new staking certificate
    cert, err := staking.NewTLSCert()
    if err != nil {
        panic(err)
    }
    
    // Save the certificate
    certOut, err := os.Create("staker.crt")
    if err != nil {
        panic(err)
    }
    defer certOut.Close()
    
    if err := pem.Encode(certOut, &pem.Block{
        Type:  "CERTIFICATE",
        Bytes: cert.CertificateBytes,
    }); err != nil {
        panic(err)
    }
    
    // Save the private key
    keyOut, err := os.OpenFile("staker.key", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    if err != nil {
        panic(err)
    }
    defer keyOut.Close()
    
    if err := pem.Encode(keyOut, &pem.Block{
        Type:  "PRIVATE KEY", 
        Bytes: cert.PrivateKeyBytes,
    }); err != nil {
        panic(err)
    }
    
    fmt.Printf("Generated NodeID: %s\n", cert.ID())
}
