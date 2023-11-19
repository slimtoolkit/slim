package kubernetes

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/slimtoolkit/slim/pkg/app/master/config"
)

type Kubectl interface {
	CpFrom(ctx context.Context, namespace, pod, container, srcPath, dstPath string) ([]byte, error)
	CpTo(ctx context.Context, namespace, pod, container, srcPath, dstPath string) ([]byte, error)
	Exec(ctx context.Context, namespace, pod, container, cmd string, args ...string) ([]byte, error)
	PortForward(ctx context.Context, namespace, pod, address, hostPort, podPort string) (*exec.Cmd, string, error)
}

type kubectl struct {
	kubeconfig string
}

var _ Kubectl = &kubectl{}

func NewKubectl(opts config.KubernetesOptions) Kubectl {
	return &kubectl{
		kubeconfig: opts.Kubeconfig,
	}
}

func (k *kubectl) CpFrom(
	ctx context.Context,
	namespace string,
	pod string,
	container string,
	srcPath string,
	dstPath string,
) ([]byte, error) {
	cmd1 := []string{
		"kubectl",
		"exec", pod,
		"--kubeconfig", k.kubeconfig,
		"--namespace", namespace,
		"--container", container,
		"--", "tar", "cf", "-", "-C", filepath.Dir(srcPath), filepath.Base(srcPath),
	}

	cmd2 := []string{"tar", "xf", "-", "-C", filepath.Dir(dstPath), filepath.Base(dstPath)}

	cmd := exec.CommandContext(
		ctx,
		"bash", "-c", strings.Join(cmd1, " ")+"|"+strings.Join(cmd2, " "),
	)
	return cmd.CombinedOutput()

	// r, w, err := os.Pipe()
	// if err != nil {
	// 	return nil, err
	// }

	// c1.Stdout = w
	// c2.Stdin = io.TeeReader(r, os.Stdout)

	// var out bytes.Buffer
	// c2.Stdout = &out
	// c2.Stderr = &out

	// if err := c1.Start(); err != nil {
	// 	return nil, err
	// }

	// if err := c2.Start(); err != nil {
	// 	return nil, err
	// }

	// if err := c1.Wait(); err != nil {
	// 	return nil, err
	// }

	// if err := w.Close(); err != nil {
	// 	return nil, err
	// }

	// if err := c2.Wait(); err != nil {
	// 	return nil, err
	// }

	// return out.Bytes(), nil
}

func (k *kubectl) CpTo(
	ctx context.Context,
	namespace string,
	pod string,
	container string,
	srcPath string,
	dstPath string,
) ([]byte, error) {
	return exec.CommandContext(
		ctx,
		"kubectl",
		"cp", srcPath, pod+":"+dstPath,
		"--kubeconfig", k.kubeconfig,
		"--namespace", namespace,
		"--container", container,
	).CombinedOutput()
}

func (k *kubectl) Exec(
	ctx context.Context,
	namespace string,
	pod string,
	container string,
	cmd string,
	args ...string,
) ([]byte, error) {
	args = append([]string{
		"kubectl",
		"exec", pod,
		"--kubeconfig", k.kubeconfig,
		"--namespace", namespace,
		"--container", container,
		"--", cmd,
	}, args...)
	return exec.Command(args[0], args[1:]...).CombinedOutput()
}

func (k *kubectl) PortForward(
	ctx context.Context,
	namespace string,
	pod string,
	address string,
	hostPort string,
	podPort string,
) (*exec.Cmd, string, error) {
	if podPort == "" {
		return nil, "", errors.New("podPort cannot be empty")
	}

	mapping := ":" + podPort
	if hostPort != "" {
		mapping = hostPort + mapping
	}

	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"--kubeconfig", k.kubeconfig,
		"--namespace", namespace,
		"--address", address,
		"port-forward", "pod/"+pod, mapping,
	)

	out, err := cmd.StdoutPipe()
	if err != nil {
		return cmd, "", err
	}

	if err := cmd.Start(); err != nil {
		return cmd, "", err
	}

	var actualHostPort int
	pattern := fmt.Sprintf("Forwarding from %s:%s -> %s", address, "%d", podPort)
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		n, err := fmt.Sscanf(scanner.Text(), pattern, &actualHostPort)
		if err == nil && n == 1 {
			return cmd, fmt.Sprintf("%d", actualHostPort), nil
		}
	}

	return cmd, "", errors.New("cannot detect host port")
}
