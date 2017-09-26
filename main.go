package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	userProcess := exec.Command(os.Args[1], os.Args[2:]...)
	userProcess.Stdout = os.Stdout
	userProcess.Stderr = os.Stderr
	err := userProcess.Run()
	must("run user process", err)
}

func must(action string, err error) {
	if err != nil {
		fmt.Printf("error %s: %s\n", action, err)
		os.Exit(1)
	}
}
