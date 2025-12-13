package main

import "hash/fnv"

func getFNVHash(blob []byte) uint64 {
	// Fast non cryptographic hash
	// https://hackernoon.com/modern-hashing-with-go-a-guide
	h := fnv.New64a()
	h.Write(blob)
	return h.Sum64()
}
