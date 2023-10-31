package sensor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func imagePull(ctx context.Context, name string) error {
	_, err := docker(ctx, "image", "pull", name)
	return err
}

func imageInspect(ctx context.Context, name string) (dockerapi.Image, error) {
	out, err := docker(ctx, "image", "inspect", name)
	if err != nil {
		return dockerapi.Image{}, err
	}

	var images []dockerapi.Image
	if err := json.Unmarshal([]byte(out), &images); err != nil {
		return dockerapi.Image{}, fmt.Errorf("cannot decode docker command output %q: %w", out, err)
	}

	if len(images) > 1 {
		return dockerapi.Image{}, fmt.Errorf("ambiguous image name %q", name)
	}

	return images[0], nil
}

func containerCreate(
	ctx context.Context,
	flags []string,
	image string,
	arg ...string,
) (string, error) {
	tail := append([]string{"create"}, flags...)
	tail = append(tail, image)
	tail = append(tail, arg...)
	return docker(ctx, "container", tail...)
}

func containerStart(ctx context.Context, contID string) error {
	_, err := docker(ctx, "container", "start", contID)
	return err
}

func containerWait(ctx context.Context, contID string) (int, error) {
	out, err := docker(ctx, "container", "wait", contID)
	if err != nil {
		return -1, err
	}

	exitCode, err := strconv.Atoi(string(out))
	if err != nil {
		return -1, fmt.Errorf("unexpected container wait output %q - expected number (exit code)", string(out))
	}
	return exitCode, nil
}

func containerKill(ctx context.Context, contID string, sig syscall.Signal) error {
	_, err := docker(ctx, "container", "kill", "-s", unix.SignalName(sig), contID)
	return err
}

func containerRemove(ctx context.Context, contID string) error {
	_, err := docker(ctx, "container", "rm", contID)
	return err
}

func containerInspect(ctx context.Context, contID string) (dockerapi.Container, error) {
	out, err := docker(ctx, "container", "inspect", contID)
	if err != nil {
		return dockerapi.Container{}, err
	}

	var conts []dockerapi.Container
	if err := json.Unmarshal([]byte(out), &conts); err != nil {
		return dockerapi.Container{}, fmt.Errorf("cannot decode docker command output %q: %w", out, err)
	}

	if len(conts) > 1 {
		return dockerapi.Container{}, fmt.Errorf("ambiguous container id/name %q", contID)
	}

	return conts[0], nil
}

func containerExec(ctx context.Context, contID string, arg ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{"container", "exec", contID}, arg...)...)

	log.Debug("Executing: ", cmd.String())

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func containerLogs(ctx context.Context, contID string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "container", "logs", contID)

	log.Debug("Executing: ", cmd.String())

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func containerCopyFrom(ctx context.Context, contID, orig, dest string) error {
	_, err := docker(ctx, "container", "cp", contID+":"+orig, dest)
	return err
}

func docker(ctx context.Context, command string, arg ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", append([]string{command}, arg...)...)
	cmd.Stderr = os.Stderr

	log.Debug("Executing: ", cmd.String())

	out, err := cmd.Output()
	if len(out) > 0 {
		return string(bytes.Trim(out, " \t\r\n")), err
	}
	return "", err
}
