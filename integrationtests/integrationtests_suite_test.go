package integrationtests_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var isolateBinPath string

var _ = SynchronizedBeforeSuite(func() []byte {
	binPath, err := gexec.Build("github.com/craigfurman/isolate")
	Expect(err).NotTo(HaveOccurred())
	return []byte(binPath)
}, func(fromFirstNode []byte) {
	isolateBinPath = string(fromFirstNode)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestIntegrationtests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integrationtests Suite")
}
