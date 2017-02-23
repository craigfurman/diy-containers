package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	if os.Args[0] == "/proc/self/exe" {
		inner()
	} else {
		outer()
	}
}

func outer() {
	rootFS := flag.String("rootFS", "", "rootFS")
	flag.Parse()
	if *rootFS == "" {
		fmt.Println("must set -rootFS")
		os.Exit(1)
	}

	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "remount"))

	cmd := exec.Command("/proc/self/exe", append([]string{*rootFS}, flag.Args()...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.Sys().(syscall.WaitStatus).ExitStatus())
		}

		must(err)
	}
}

func inner() {
	rootFS := os.Args[1]

	must(os.MkdirAll(filepath.Join(rootFS, "oldrootfs"), 0700))
	must(syscall.Mount(rootFS, rootFS, "", syscall.MS_BIND, ""))
	must(os.Chdir(rootFS))
	must(syscall.PivotRoot(".", "oldrootfs"))
	must(os.Chdir("/"))
	must(syscall.Mount("proc", "/proc", "proc", 0, ""))
	must(syscall.Unmount("/oldrootfs", syscall.MNT_DETACH))

	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println(err)
			os.Exit(exitErr.Sys().(syscall.WaitStatus).ExitStatus())
		}

		must(err)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
