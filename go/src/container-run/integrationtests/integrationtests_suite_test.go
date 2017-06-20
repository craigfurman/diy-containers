package integrationtests

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var containerRunBinPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	externalBinPath := os.Getenv("DIY_CONTAINER_BIN_PATH")
	if externalBinPath != "" {
		return []byte(externalBinPath)
	}

	binPath, err := gexec.Build("container-run")
	Expect(err).NotTo(HaveOccurred())
	return []byte(binPath)
}, func(data []byte) {
	containerRunBinPath = string(data)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
