package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	coresToUse := flag.String("cores", "", "OPTIONAL: which cores to use? examples: '0', or '0-1'")
	flag.Parse()
	if *rootFS == "" {
		fmt.Println("must set -rootFS")
		os.Exit(1)
	}
	cores := fmt.Sprintf("0-%d", runtime.NumCPU()-1)
	if *coresToUse != "" {
		cores = *coresToUse
	}

	cmd := exec.Command("/proc/self/exe", append([]string{"child", *rootFS, cores}, flag.Args()...)...)
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
	cores := os.Args[3]

	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE, "remount"))
	must(os.MkdirAll(filepath.Join(rootFS, "oldrootfs"), 0700))
	must(syscall.Mount(rootFS, rootFS, "", syscall.MS_BIND, ""))
	must(os.Chdir(rootFS))
	must(syscall.PivotRoot(".", "oldrootfs"))
	must(os.Chdir("/"))
	must(syscall.Mount("", "/proc", "proc", 0, ""))
	must(syscall.Unmount("/oldrootfs", syscall.MNT_DETACH))

	must(syscall.Mount("cgroup_root", "/sys/fs/cgroup", "tmpfs", 0, ""))
	must(os.MkdirAll("/sys/fs/cgroup/cpuset", 0700))
	must(syscall.Mount("cpuset", "/sys/fs/cgroup/cpuset", "cgroup", 0, "cpuset"))
	must(os.MkdirAll("/sys/fs/cgroup/cpuset/container", 0700))
	must(ioutil.WriteFile("/sys/fs/cgroup/cpuset/container/cpuset.mems", []byte("0"), 0600))
	must(ioutil.WriteFile("/sys/fs/cgroup/cpuset/container/cpuset.cpus", []byte(cores), 0600))
	must(ioutil.WriteFile("/sys/fs/cgroup/cpuset/container/tasks", []byte(fmt.Sprintf("%d", os.Getpid())), 0600))

	cmd := exec.Command(os.Args[4], os.Args[5:]...)
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
