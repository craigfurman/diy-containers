package integrationtests_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("containerising processes", func() {
	var (
		vmDir string
	)

	runCommandInVM := func(shellCmd string) (int, string, error) {
		containerCmd := exec.Command("vagrant", "ssh", "-c", shellCmd)
		containerCmd.Dir = vmDir
		var stdout bytes.Buffer
		containerCmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
		containerCmd.Stderr = GinkgoWriter
		if err := containerCmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.Sys().(syscall.WaitStatus).ExitStatus(), stdout.String(), nil
			}

			return 0, "", err
		}

		return 0, stdout.String(), nil
	}

	runCommandInContainer := func(containerCmd ...string) (int, string, error) {
		shellCmd := "sudo /go/bin/linux_amd64/container-run -rootFS /root/rootfs/jessie"
		for _, term := range containerCmd {
			shellCmd = fmt.Sprintf("%s '%s'", shellCmd, term)
		}
		return runCommandInVM(shellCmd)
	}

	BeforeEach(func() {
		vmDir = os.Getenv("VM_DIR")
		Expect(vmDir).NotTo(BeEmpty())
	})

	It("puts the process in a UTS namespace", func() {
		exitStatus, stdout, err := runCommandInContainer("bash", "-c", "hostname new-hostname && hostname")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(Equal("new-hostname\n"))
	})
})
