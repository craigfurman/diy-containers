package integrationtests

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("containerising processes", func() {
	It("runs the process in a UTS namespace", func() {
		exitStatus, output, err := runCommandInContainer(true, 0, "/bin/bash", "-c", "hostname new-hostname && hostname")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).To(Equal("new-hostname\n"))
		exitStatus, output, err = runCommand("hostname")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).To(Equal("ubuntu-xenial\n"))
	})

	It("runs the process in a PID namespace", func() {
		exitStatus, output, err := runCommandInContainer(true, 0, "/bin/ps", "-lfp", "1")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).To(ContainSubstring("ps -lfp 1"))
	})

	It("runs the process in a mount namespace", func() {
		exitStatus, output, err := runCommandInContainer(true, 0, "/bin/bash", "-c", "mount -t tmpfs tmpfs /tmp && cat /proc/self/mounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).To(ContainSubstring("tmpfs /tmp"))
		exitStatus, output, err = runCommand("cat", "/proc/self/mounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).NotTo(ContainSubstring("tmpfs /tmp"))
	})

	It("runs the process with a Debian rootFS", func() {
		exitStatus, output, err := runCommandInContainer(true, 0, "/bin/cat", "/etc/os-release")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		Expect(output).To(ContainSubstring("Debian GNU/Linux 8 (jessie)"))
	})

	It("runs the process in a unique rootFS", func() {
		exitStatus, _, err := runCommandInContainer(true, 0, "/usr/bin/touch", "/tmp/a-file")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
		exitStatus, _, err = runCommandInContainer(true, 0, "/usr/bin/stat", "/tmp/a-file")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(1))
	})

	It("can make mknod system calls when privileged", func() {
		exitStatus, _, err := runCommandInContainer(true, 0, "/bin/mknod", "/tmp/node", "b", "7", "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).To(Equal(0))
	})

	It("cannot make mknod system calls when unprivileged", func() {
		exitStatus, output, err := runCommandInContainer(false, 0, "/bin/mknod", "/tmp/node", "b", "7", "0")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).NotTo(Equal(0))
		Expect(output).To(ContainSubstring("mknod: '/tmp/node': Operation not permitted"))
	})

	It("can restrict the max memory of a process", func() {
		exitStatus, output, err := runCommandInContainer(false, 20, "/usr/bin/stress", "--vm", "1", "--vm-bytes", "512M", "--vm-keep", "-t", "1")
		Expect(err).NotTo(HaveOccurred())
		Expect(exitStatus).NotTo(Equal(0))
		Expect(output).To(MatchRegexp(`worker \d got signal 9`))
	})
})

func runCommand(exe string, args ...string) (int, string, error) {
	cmd := exec.Command(exe, args...)
	var output bytes.Buffer
	cmd.Stdout = io.MultiWriter(&output, GinkgoWriter)
	cmd.Stderr = io.MultiWriter(&output, GinkgoWriter)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitStatus := exitErr.Sys().(syscall.WaitStatus).ExitStatus()
			return exitStatus, output.String(), nil
		}

		return 0, "", err
	}

	return 0, output.String(), nil
}

func runCommandInContainer(privileged bool, maxMemoryMB int, containerCmd ...string) (int, string, error) {
	args := []string{"-rootFS", "/root/rootfs/jessie"}
	if privileged {
		args = append(args, "-privileged")
	}
	if maxMemoryMB != 0 {
		args = append(args, "-maxMemoryMB", fmt.Sprintf("%d", maxMemoryMB))
	}
	args = append(args, containerCmd...)
	return runCommand(containerRunBinPath, args...)
}
