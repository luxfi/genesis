# Decode block numbers from hashes
hashes = [
    "0000006c3a436500b20c0c80f5dae66e1233d84da4ddd5af2987cfdb1562eb9f",
    "0000010214efc2d0f09b4b0bce1f1f5af7df428471031886bff73119c45cdcbc",
    "000002d7a7e5d7bb05b43c21aef385b934c61d3a7605c0829c35defb490a651c",
]

for h in hashes:
    # First 8 bytes
    first_8 = int(h[:16], 16)
    # First 4 bytes  
    first_4 = int(h[:8], 16)
    # First 3 bytes
    first_3 = int(h[:6], 16)
    
    print(f"Hash: {h}")
    print(f"  First 3 bytes: {first_3}")
    print(f"  First 4 bytes: {first_4}")
    print(f"  First 8 bytes: {first_8}")
    print()
