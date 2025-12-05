package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"khelper/pkg/config"
	"khelper/pkg/k8s"
	"khelper/pkg/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	namespace  string
	deployment string
	pod        string
	container  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "khelper",
		Short: "Interactive Kubernetes deployment helper",
		Long:  `khelper is an interactive CLI tool that simplifies Kubernetes deployment management with a modern terminal UI.`,
		RunE:  runInteractive,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace")
	rootCmd.PersistentFlags().StringVarP(&deployment, "deployment", "d", "", "Deployment name")
	rootCmd.PersistentFlags().StringVarP(&pod, "pod", "p", "", "Pod name")
	rootCmd.PersistentFlags().StringVarP(&container, "container", "c", "", "Container name")

	// Subcommands
	rootCmd.AddCommand(logsCmd())
	rootCmd.AddCommand(shellCmd())
	rootCmd.AddCommand(scaleCmd())
	rootCmd.AddCommand(portForwardCmd())
	rootCmd.AddCommand(updateImageCmd())

	// Silence Cobra's default error printing - we handle it ourselves
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runInteractive(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override namespace from flag if provided
	if namespace != "" {
		cfg.LastNamespace = namespace
	}

	// Try to create k8s client, but don't fail if no kubeconfig exists
	// The UI will prompt user to select/enter a kubeconfig path
	var k8sClient *k8s.Client
	var clientErr error
	if cfg.KubeConfig != "" {
		k8sClient, clientErr = k8s.NewClientWithConfig(cfg.KubeConfig)
	} else {
		k8sClient, clientErr = k8s.NewClient()
	}

	// Create model - it will handle nil client by showing kubeconfig selection
	model := ui.NewModel(cfg, k8sClient, clientErr)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Handle post-TUI actions
	m := finalModel.(ui.Model)
	return handlePostTUIAction(m, k8sClient)
}

func handlePostTUIAction(m ui.Model, k8sClient *k8s.Client) error {
	if m.GetCommand() == nil {
		return nil
	}

	switch m.GetCommand().Name {
	case "shell":
		// Empty string lets the Shell function auto-detect the best shell
		return ui.RunShell(k8sClient, m.GetNamespace(), m.GetPod(), m.GetContainer(), "")
	case "logs-follow":
		return ui.RunLogs(k8sClient, m.GetNamespace(), m.GetPod(), m.GetContainer(), true)
	case "port-forward":
		parts := strings.Split(m.GetInputValue(), ":")
		if len(parts) == 2 {
			local, _ := strconv.Atoi(parts[0])
			remote, _ := strconv.Atoi(parts[1])
			return ui.RunPortForward(k8sClient, m.GetNamespace(), m.GetPod(), local, remote)
		}
	}

	return nil
}

func logsCmd() *cobra.Command {
	var follow bool
	var tailLines int64

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View container logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if namespace == "" || deployment == "" || pod == "" || container == "" {
				return fmt.Errorf("namespace, deployment, pod, and container are required")
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return err
			}

			return ui.RunLogs(k8sClient, namespace, pod, container, follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int64VarP(&tailLines, "tail", "t", 100, "Number of lines to show")

	return cmd
}

func shellCmd() *cobra.Command {
	var shell string

	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Open shell in container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if namespace == "" || pod == "" || container == "" {
				return fmt.Errorf("namespace, pod, and container are required")
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return err
			}

			return ui.RunShell(k8sClient, namespace, pod, container, shell)
		},
	}

	cmd.Flags().StringVarP(&shell, "shell", "s", "/bin/sh", "Shell to use")

	return cmd
}

func scaleCmd() *cobra.Command {
	var replicas int32

	cmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if namespace == "" || deployment == "" {
				return fmt.Errorf("namespace and deployment are required")
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if err := k8sClient.ScaleDeployment(ctx, namespace, deployment, replicas); err != nil {
				return err
			}

			fmt.Printf("Scaled %s to %d replicas\n", deployment, replicas)
			return nil
		},
	}

	cmd.Flags().Int32VarP(&replicas, "replicas", "r", 1, "Number of replicas")
	cmd.MarkFlagRequired("replicas")

	return cmd
}

func portForwardCmd() *cobra.Command {
	var localPort, remotePort int

	cmd := &cobra.Command{
		Use:   "port-forward",
		Short: "Forward port to pod",
		RunE: func(cmd *cobra.Command, args []string) error {
			if namespace == "" || pod == "" {
				return fmt.Errorf("namespace and pod are required")
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return err
			}

			return ui.RunPortForward(k8sClient, namespace, pod, localPort, remotePort)
		},
	}

	cmd.Flags().IntVarP(&localPort, "local", "l", 8080, "Local port")
	cmd.Flags().IntVarP(&remotePort, "remote", "r", 80, "Remote port")

	return cmd
}

func updateImageCmd() *cobra.Command {
	var image string

	cmd := &cobra.Command{
		Use:   "update-image",
		Short: "Update container image",
		RunE: func(cmd *cobra.Command, args []string) error {
			if namespace == "" || deployment == "" || container == "" || image == "" {
				return fmt.Errorf("namespace, deployment, container, and image are required")
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if err := k8sClient.UpdateImage(ctx, namespace, deployment, container, image); err != nil {
				return err
			}

			fmt.Printf("Updated %s image to %s\n", container, image)
			return nil
		},
	}

	cmd.Flags().StringVarP(&image, "image", "i", "", "New image")
	cmd.MarkFlagRequired("image")

	return cmd
}
