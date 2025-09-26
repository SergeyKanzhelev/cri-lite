package policy_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Policy Suite")
}

func createSocket(sockDir string) string {
	f, err := os.CreateTemp(sockDir, "socket-*.sock")
	Expect(err).NotTo(HaveOccurred())

	path := f.Name()
	Expect(f.Close()).To(Succeed())
	Expect(os.Remove(path)).To(Succeed())

	return path
}
