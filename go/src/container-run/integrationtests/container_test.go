package integrationtests

import (
	"bytes"
	"io"
	"os/exec"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("containerising processes", func() {
	It("runs the process in a UTS namespace", func() {
		exitStatus, stdout, err := runCommandInContainer("bash", "-c", "hostname new-hostname && hostname")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(Equal("new-hostname\n"))
		exitStatus, stdout, err = runCommand("hostname")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(Equal("ubuntu-xenial\n"))
	})

	It("runs the process in a PID namespace", func() {
		exitStatus, stdout, err := runCommandInContainer("ps", "-lfp", "1")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(ContainSubstring("/proc/self/exe /root/rootfs/jessie ps -lfp 1"))
	})

	It("runs the process in a mount namespace", func() {
		exitStatus, stdout, err := runCommandInContainer("bash", "-c", "mount -t tmpfs tmpfs /tmp && cat /proc/self/mounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(ContainSubstring("tmpfs /tmp"))
		exitStatus, stdout, err = runCommand("cat", "/proc/self/mounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).NotTo(ContainSubstring("tmpfs /tmp"))
	})

	It("runs the process with a Debian rootFS", func() {
		exitStatus, stdout, err := runCommandInContainer("cat", "/etc/os-release")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(stdout).To(ContainSubstring("Debian GNU/Linux 8 (jessie)"))
	})
})

func runCommand(exe string, args ...string) (int, string, error) {
	cmd := exec.Command(exe, args...)
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitStatus := exitErr.Sys().(syscall.WaitStatus).ExitStatus()
			return exitStatus, stdout.String(), nil
		}

		return 0, "", err
	}

	return 0, stdout.String(), nil
}

func runCommandInContainer(containerCmd ...string) (int, string, error) {
	args := append([]string{"-rootFS", "/root/rootfs/jessie"}, containerCmd...)
	return runCommand(containerRunBinPath, args...)
}
