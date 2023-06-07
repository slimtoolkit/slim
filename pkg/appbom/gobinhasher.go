//go:build ignore
// +build ignore

package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	fullPath, err := getGoExeFullPath()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("found go binary: %s\n", fullPath)

	hash, err := hashFile(fullPath)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	data := fmt.Sprintf("sha256:%s", hash)
	fmt.Printf("saving go binary hash: %s\n", data)
	err = os.WriteFile("gobinhash", []byte(data), 0644)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}

const (
	goBinName    = "go"
	goEnvCmd     = "env"
	goRootEnvVar = "GOROOT"
	goBinPathPat = "%s/bin/%s"
)

func getGoExeFullPath() (string, error) {
	output, err := exec.Command(goBinName, goEnvCmd, goRootEnvVar).Output()
	if err != nil {
		return "", err
	}

	goRoot := string(output[:len(output)-1]) // removing the newline
	fullPath := fmt.Sprintf(goBinPathPat, goRoot, goBinName)

	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, nil
	}

	return exec.LookPath(goBinName)
}

func hashFile(fullPath string) (string, error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	hash := hasher.Sum(nil)
	return fmt.Sprintf("%x", hash), nil
}
