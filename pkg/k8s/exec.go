package k8s

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecOptions holds options for executing commands in a container
type ExecOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	TTY           bool
}

// Exec executes a command in a container
func (c *Client) Exec(ctx context.Context, opts ExecOptions) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.ContainerName,
			Command:   opts.Command,
			Stdin:     opts.Stdin != nil,
			Stdout:    opts.Stdout != nil,
			Stderr:    opts.Stderr != nil,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	streamOpts := remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    opts.TTY,
	}

	return executor.StreamWithContext(ctx, streamOpts)
}

// Shell opens an interactive shell in a container
// It tries multiple shells in order: the specified shell, then /bin/bash, /bin/sh, /bin/ash, sh
func (c *Client) Shell(ctx context.Context, namespace, podName, containerName string, shell string) error {
	// List of shells to try in order of preference
	shells := []string{}

	if shell != "" {
		shells = append(shells, shell)
	}

	// Add common shells
	defaultShells := []string{"/bin/bash", "/bin/sh", "/bin/ash", "sh", "ash"}
	for _, s := range defaultShells {
		// Avoid duplicates
		found := false
		for _, existing := range shells {
			if existing == s {
				found = true
				break
			}
		}
		if !found {
			shells = append(shells, s)
		}
	}

	for _, sh := range shells {
		err := c.Exec(ctx, ExecOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			Command:       []string{sh},
			Stdin:         os.Stdin,
			Stdout:        os.Stdout,
			Stderr:        os.Stderr,
			TTY:           true,
		})

		if err == nil {
			return nil
		}

		// Check if the error is about shell not found - try next shell
		errStr := err.Error()
		if contains(errStr, "no such file or directory") ||
			contains(errStr, "executable file not found") ||
			contains(errStr, "not found") {
			continue
		}

		// For other errors (like connection issues), don't try more shells
		return err
	}

	return fmt.Errorf("no shell available in container.\n\nThis container appears to be a minimal/distroless image without a shell.\nYou can still use 'logs' to view container output.\n\nTried shells: %v", shells)
}

// CheckShellAvailable checks if any shell is available in the container without opening an interactive session
func (c *Client) CheckShellAvailable(ctx context.Context, namespace, podName, containerName string) (string, error) {
	shells := []string{"/bin/bash", "/bin/sh", "/bin/ash", "sh", "ash"}

	for _, sh := range shells {
		// Try running "exit 0" with the shell to check if it exists
		err := c.Exec(ctx, ExecOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			Command:       []string{sh, "-c", "exit 0"},
			Stdin:         nil,
			Stdout:        nil,
			Stderr:        nil,
			TTY:           false,
		})

		if err == nil {
			return sh, nil
		}

		// Check if the error is about shell not found - try next shell
		errStr := err.Error()
		if contains(errStr, "no such file or directory") ||
			contains(errStr, "executable file not found") ||
			contains(errStr, "not found") {
			continue
		}

		// For other errors (like connection issues), return the error
		return "", err
	}

	return "", fmt.Errorf("no shell available in container.\n\nThis container appears to be a minimal/distroless image without a shell.\nYou can still use 'logs' to view container output.\n\nTried shells: %v", shells)
}

// contains checks if a string contains a substring (case insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > 0 && (containsAt(s, substr, 0) || contains(s[1:], substr)))
}

func containsAt(s, substr string, pos int) bool {
	if pos+len(substr) > len(s) {
		return false
	}
	for i := 0; i < len(substr); i++ {
		sc := s[pos+i]
		uc := substr[i]
		// Simple lowercase comparison for ASCII
		if sc >= 'A' && sc <= 'Z' {
			sc = sc + 32
		}
		if uc >= 'A' && uc <= 'Z' {
			uc = uc + 32
		}
		if sc != uc {
			return false
		}
	}
	return true
}
