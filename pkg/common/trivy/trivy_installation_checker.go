package trivy

import (
	"bytes"
	"fmt"
	"os/exec"
)

func CheckTrivyInstalled() bool {
	cmd := exec.Command("trivy", "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		fmt.Println("Trivy is not installed or not found in PATH:", err)
		fmt.Println(out.String())
		return false
	}
	return true
}
