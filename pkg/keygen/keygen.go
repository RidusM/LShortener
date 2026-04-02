package keygen

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

type Generator interface {
	Generate(size int) (string, error)
}

type base62Generator struct {
	charset string
}

func NewBase62Generator() Generator {
	return &base62Generator{
		charset: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	}
}

func (g *base62Generator) Generate(size int) (string, error) {
	b := make([]byte, size)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(g.charset))))
		if err != nil {
			return "", fmt.Errorf("pkg.keygen.generate: %w", err)
		}
		b[i] = g.charset[idx.Int64()]
	}
	return string(b), nil
}
