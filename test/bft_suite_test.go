package test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBFT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BFT Consensus Suite", Label("BFT"))
}