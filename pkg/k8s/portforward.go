package k8s

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// PortForwardOptions holds options for port forwarding
type PortForwardOptions struct {
	Namespace  string
	PodName    string
	LocalPort  int
	RemotePort int
}

// PortForward starts port forwarding to a pod
func (c *Client) PortForward(ctx context.Context, opts PortForwardOptions) error {
	url := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(opts.Namespace).
		Name(opts.PodName).
		SubResource("portforward").
		URL()

	return c.portForward(ctx, url, opts)
}

func (c *Client) portForward(ctx context.Context, url *url.URL, opts PortForwardOptions) error {
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	ports := []string{fmt.Sprintf("%d:%d", opts.LocalPort, opts.RemotePort)}
	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := pf.ForwardPorts(); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-readyChan:
		fmt.Printf("Port forwarding is ready. Forwarding %d -> %d\n", opts.LocalPort, opts.RemotePort)
		fmt.Println("Press Ctrl+C to stop...")
	case err := <-errChan:
		return err
	case <-ctx.Done():
		close(stopChan)
		return ctx.Err()
	}

	select {
	case <-sigChan:
		fmt.Println("\nStopping port forward...")
		close(stopChan)
	case err := <-errChan:
		return err
	case <-ctx.Done():
		close(stopChan)
		return ctx.Err()
	}

	return nil
}
