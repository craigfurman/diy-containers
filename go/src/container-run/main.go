package main

import (
	"flag"
	"fmt"
	"io"
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

	cowRootFS, err := ioutil.TempDir("", "container-run")
	must(err)
	defer func() {
		must(os.RemoveAll(cowRootFS))
	}()
	must(createUniqueRootFS(*rootFS, cowRootFS))

	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "remount"))

	cmd := exec.Command("/proc/self/exe", append([]string{cowRootFS}, flag.Args()...)...)
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

func createUniqueRootFS(rootFS, cowRootFS string) error {
	return filepath.Walk(rootFS, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(rootFS, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(cowRootFS, relativePath)

		if info.IsDir() {
			return os.MkdirAll(newPath, info.Mode())
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, newPath)
		}

		if info.Mode()&os.ModeDevice != 0 {
			// Don't bother setting up devices for the container
			return nil
		}

		originalFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer originalFile.Close()
		newFile, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		defer newFile.Close()
		if _, err := io.Copy(newFile, originalFile); err != nil {
			return err
		}

		return nil
	})
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
