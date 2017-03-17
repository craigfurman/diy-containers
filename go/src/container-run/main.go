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
	must(os.Mkdir(oldRootFS, 0700))
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
	lowerLayer := rootFS
	if chownTo != 0 {
		var err error
		lowerLayer, err = createUnprivilegedRootFS(rootFS, chownTo)
		if err != nil {
			return "", err
		}
	}

	if err := os.Chown(cowRootFS, chownTo, chownTo); err != nil {
		return "", nil
	}

	containerRootFS := filepath.Join(cowRootFS, "union")
	workDir := filepath.Join(cowRootFS, "work")
	upperDir := filepath.Join(cowRootFS, "upper")
	for _, dir := range []string{containerRootFS, workDir, upperDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return "", err
		}
		if err := os.Chown(dir, chownTo, chownTo); err != nil {
			return "", nil
		}
	}
	if err := mountOverlay(lowerLayer, upperDir, workDir, containerRootFS); err != nil {
		return "", err
	}
	return containerRootFS, nil
}

func createUnprivilegedRootFS(rootFS string, uid int) (string, error) {
	fssDir := filepath.Dir(rootFS)
	fsName := filepath.Base(rootFS)
	unprivilegedRootFSPath := filepath.Join(fssDir, fsName+"-unprivileged")

	lockPath := filepath.Join(fssDir, fsName+"-chownlock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer func() {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
	}()

	if _, err := os.Stat(unprivilegedRootFSPath); err == nil {
		return unprivilegedRootFSPath, nil
	}

	if err := filepath.Walk(rootFS, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(rootFS, path)
		if err != nil {
			return err
		}
		newPath := filepath.Join(unprivilegedRootFSPath, relativePath)

		if info.IsDir() {
			if err := os.MkdirAll(newPath, info.Mode()); err != nil {
				return err
			}
			return os.Chown(newPath, uid, uid)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, newPath); err != nil {
				return err
			}
			return os.Lchown(newPath, uid, uid)
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

		return os.Chown(newPath, uid, uid)
	}); err != nil {
		return "", err
	}

	return unprivilegedRootFSPath, nil
}

func mountOverlay(lowerDir, upperDir, workDir, unionDir string) error {
	overlayMountOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
	return syscall.Mount("overlay", unionDir, "overlay", 0, overlayMountOpts)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
