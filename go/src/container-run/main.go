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
	switch os.Args[1] {
	case "child":
		child()
	default:
		parent()
	}
}

func parent() {
	rootFS := flag.String("rootFS", "", "rootFS")
	flag.Parse()
	if *rootFS == "" {
		fmt.Println("must set -rootFS")
		os.Exit(1)
	}

	cmd := exec.Command("/proc/self/exe", append([]string{"child", *rootFS}, flag.Args()...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.Sys().(syscall.WaitStatus).ExitStatus())
		}

		fmt.Println("ERROR running container", err)
		os.Exit(1)
	}
}

func child() {
	rootFS := os.Args[2]
	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE, ""))
	must(os.MkdirAll(filepath.Join(rootFS, "oldrootfs"), 0700))
	must(syscall.Mount(rootFS, rootFS, "", syscall.MS_BIND, ""))
	must(os.Chdir(rootFS))
	must(syscall.PivotRoot(".", "oldrootfs"))
	must(os.Chdir("/"))
	must(syscall.Mount("", "/proc", "proc", 0, ""))
	must(syscall.Unmount("/oldrootfs", syscall.MNT_DETACH))

	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println(err)
			os.Exit(exitErr.Sys().(syscall.WaitStatus).ExitStatus())
		}

		fmt.Println("ERROR running process in container", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
