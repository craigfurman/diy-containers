package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	if os.Args[0] == "/proc/self/exe" {
		inner()
	} else {
		os.Exit(outer())
	}
}

func outer() int {
	rootFS := flag.String("rootFS", "", "rootFS")
	privileged := flag.Bool("privileged", false, "if true, user namespace is not used")
	flag.Parse()
	if *rootFS == "" {
		fmt.Println("must set -rootFS")
		return 1
	}

	cowRootFS, err := ioutil.TempDir("", "container-run")
	must(err)

	mappingSize := 100000
	chownTo := mappingSize
	if *privileged {
		chownTo = 0
	}
	containerRootFSPath, err := createUniqueRootFS(*rootFS, cowRootFS, chownTo)
	must(err)
	defer func() {
		must(syscall.Unmount(containerRootFSPath, 0))
		must(os.RemoveAll(cowRootFS))
	}()

	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "remount"))

	cmd := exec.Command("/proc/self/exe", append([]string{containerRootFSPath}, flag.Args()...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
	}
	if !*privileged {
		cmd.SysProcAttr.Cloneflags = cmd.SysProcAttr.Cloneflags | syscall.CLONE_NEWUSER
		mapping := []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      mappingSize,
				Size:        1,
			},
			{
				ContainerID: 1,
				HostID:      1,
				Size:        mappingSize - 1,
			},
		}
		cmd.SysProcAttr.UidMappings = mapping
		cmd.SysProcAttr.GidMappings = mapping
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: 0, Gid: 0}
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.Sys().(syscall.WaitStatus).ExitStatus()
		}

		must(err)
	}

	return 0
}

func inner() {
	rootFS := os.Args[1]

	oldRootFS := filepath.Join(rootFS, "oldrootfs")
	must(os.MkdirAll(oldRootFS, 0700))
	must(syscall.Mount(rootFS, rootFS, "", syscall.MS_BIND, ""))
	must(syscall.PivotRoot(rootFS, oldRootFS))
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

func createUniqueRootFS(rootFS, cowRootFS string, chownTo int) (string, error) {
	containerRootFS := filepath.Join(cowRootFS, "union")
	workDir := filepath.Join(cowRootFS, "work")
	upperDir := filepath.Join(cowRootFS, "upper")
	for _, dir := range []string{containerRootFS, workDir, upperDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return "", err
		}
	}
	overlayMountOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", rootFS, upperDir, workDir)
	if err := syscall.Mount("overlay", containerRootFS, "overlay", 0, overlayMountOpts); err != nil {
		return "", err
	}
	return containerRootFS, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
