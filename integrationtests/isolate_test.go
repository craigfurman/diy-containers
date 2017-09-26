package integrationtests_test

import (
	"bytes"
	"io"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Isolate", func() {
	It("runs the user process, forwarding stdout", func() {
		var stdout bytes.Buffer
		cmd := exec.Command(isolateBinPath, "echo", "hello")
		cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
		cmd.Stderr = GinkgoWriter
		Expect(cmd.Run()).To(Succeed())
		Expect(stdout.String()).To(Equal("hello\n"))
	})
})
