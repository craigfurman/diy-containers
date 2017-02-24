package integrationtests

import (
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	installCmd := exec.Command("go", "install", "container-run")
	installCmd.Env = append(os.Environ(), "GOOS=linux")
	installCmd.Stdout = GinkgoWriter
	installCmd.Stderr = GinkgoWriter
	Expect(installCmd.Run()).To(Succeed())
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
