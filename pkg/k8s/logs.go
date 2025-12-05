package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// LogOptions holds options for streaming logs
type LogOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Follow        bool
	TailLines     int64
	Previous      bool
}

// StreamLogs streams logs from a container
func (c *Client) StreamLogs(ctx context.Context, opts LogOptions, output io.Writer) error {
	podLogOpts := &corev1.PodLogOptions{
		Container: opts.ContainerName,
		Follow:    opts.Follow,
		Previous:  opts.Previous,
	}

	if opts.TailLines > 0 {
		podLogOpts.TailLines = &opts.TailLines
	}

	req := c.clientset.CoreV1().Pods(opts.Namespace).GetLogs(opts.PodName, podLogOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to get log stream: %w", err)
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if _, err := output.Write(line); err != nil {
				return err
			}
		}
	}
}

// GetLogs returns logs from a container as a string
func (c *Client) GetLogs(ctx context.Context, opts LogOptions) (string, error) {
	podLogOpts := &corev1.PodLogOptions{
		Container: opts.ContainerName,
		Follow:    false,
		Previous:  opts.Previous,
	}

	if opts.TailLines > 0 {
		podLogOpts.TailLines = &opts.TailLines
	}

	req := c.clientset.CoreV1().Pods(opts.Namespace).GetLogs(opts.PodName, podLogOpts)
	result, err := req.Do(ctx).Raw()
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}

	return string(result), nil
}
