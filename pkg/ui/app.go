package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"khelper/pkg/config"
	"khelper/pkg/k8s"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppState represents the current state of the application
type AppState int

const (
	StateSelectKubeConfig AppState = iota
	StateSelectNamespace
	StateSelectDeployment
	StateSelectCommand
	StateSelectPod
	StateSelectContainer
	StateSelectAssetFolder
	StateInputValue
	StateExecuting
	StateShowResult
	StateViewLogs
)

// Command represents available commands
type Command struct {
	Name           string
	Description    string
	NeedsPod       bool
	NeedsContainer bool
	NeedsInput     bool
	InputPrompt    string
}

var AvailableCommands = []Command{
	{Name: "logs", Description: "View container logs", NeedsPod: true, NeedsContainer: true},
	{Name: "logs-follow", Description: "Follow container logs", NeedsPod: true, NeedsContainer: true},
	{Name: "shell", Description: "Open shell (auto-detects bash/sh/ash)", NeedsPod: true, NeedsContainer: true},
	{Name: "fast-deploy", Description: "Deploy local dist to /app/assets", NeedsPod: true, NeedsContainer: true},
	{Name: "scale", Description: "Scale deployment", NeedsInput: true, InputPrompt: "Enter replica count:"},
	{Name: "update-image", Description: "Update container image", NeedsContainer: true, NeedsInput: true, InputPrompt: "Enter new image:"},
	{Name: "port-forward", Description: "Forward port to pod", NeedsPod: true, NeedsInput: true, InputPrompt: "Enter ports (local:remote):"},
	{Name: "rollback", Description: "Rollback deployment", NeedsInput: true, InputPrompt: "Enter revision number:"},
	{Name: "set-env", Description: "Set environment variable", NeedsContainer: true, NeedsInput: true, InputPrompt: "Enter KEY=VALUE:"},
	{Name: "list-env", Description: "List environment variables", NeedsContainer: true},
	{Name: "list-pods", Description: "List all pods"},
	{Name: "list-revisions", Description: "List deployment revisions"},
	{Name: "ingress", Description: "Show related ingresses"},
	{Name: "describe", Description: "Describe deployment"},
}

// Messages
type (
	NamespacesLoadedMsg struct {
		namespaces []string
		err        error
	}
	DeploymentsLoadedMsg struct {
		deployments []string
		err         error
	}
	PodsLoadedMsg struct {
		pods []string
		err  error
	}
	ContainersLoadedMsg struct {
		containers []string
		err        error
	}
	CommandResultMsg struct {
		result string
		err    error
	}
	ExecCompleteMsg struct {
		err error
	}
	LogsLoadedMsg struct {
		logs string
		err  error
	}
	LogLineMsg struct {
		line string
	}
	LogStreamEndMsg struct {
		err error
	}
	KubeConfigsLoadedMsg struct {
		configs []string
		err     error
	}
	KubeConfigChangedMsg struct {
		client *k8s.Client
		path   string
		err    error
	}
	AssetFoldersLoadedMsg struct {
		folders []string
		err     error
	}
	FastDeployCompleteMsg struct {
		result string
		err    error
	}
)

// Model is the main application model
type Model struct {
	config     *config.Config
	k8sClient  *k8s.Client
	state      AppState
	prevStates []AppState

	kubeconfig  string
	namespace   string
	deployment  string
	command     *Command
	pod         string
	container   string
	inputValue  string
	assetFolder string

	kcSelector    FuzzyList
	nsSelector    FuzzyList
	depSelector   FuzzyList
	cmdSelector   FuzzyList
	podSelector   FuzzyList
	contSelector  FuzzyList
	assetSelector FuzzyList
	valueInput    textinput.Model
	logViewer     LogViewer

	result       string
	err          error
	width        int
	height       int
	streaming    bool
	streamCtx    context.Context
	cancelStream context.CancelFunc

	showNamespaceChange  bool
	showKubeConfigChange bool
	initialClientErr     error
}

// NewModel creates a new application model
func NewModel(cfg *config.Config, client *k8s.Client, clientErr error) Model {
	valueInput := textinput.New()
	valueInput.CharLimit = 200
	valueInput.Width = 50
	valueInput.PromptStyle = PromptStyle
	valueInput.TextStyle = BaseStyle

	m := Model{
		config:           cfg,
		k8sClient:        client,
		initialClientErr: clientErr,
		namespace:        cfg.LastNamespace,
		kcSelector:       NewFuzzyList("Select Kubeconfig"),
		nsSelector:       NewFuzzyList("Select Namespace"),
		depSelector:      NewFuzzyList("Select Deployment"),
		cmdSelector:      NewFuzzyList("Select Command"),
		podSelector:      NewFuzzyList("Select Pod"),
		contSelector:     NewFuzzyList("Select Container"),
		assetSelector:    NewFuzzyList("Select Asset Folder"),
		valueInput:       valueInput,
		logViewer:        NewLogViewer(),
	}

	// Get kubeconfig path if client exists
	if client != nil {
		m.kubeconfig = client.GetKubeConfigPath()
	}

	// Set up command list
	cmdNames := make([]string, len(AvailableCommands))
	for i, cmd := range AvailableCommands {
		cmdNames[i] = fmt.Sprintf("%s - %s", cmd.Name, cmd.Description)
	}
	m.cmdSelector.SetItems(cmdNames)

	// Determine initial state - if no client, force kubeconfig selection
	if client == nil {
		m.state = StateSelectKubeConfig
		m.showKubeConfigChange = true
	} else if m.namespace == "" {
		m.state = StateSelectNamespace
	} else {
		m.state = StateSelectDeployment
	}

	return m
}

func (m Model) Init() tea.Cmd {
	// If no client, load kubeconfig options
	if m.k8sClient == nil {
		return m.loadKubeConfigs()
	}
	if m.namespace == "" {
		return m.loadNamespaces()
	}
	return m.loadDeployments()
}

func (m *Model) loadNamespaces() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		namespaces, err := m.k8sClient.ListNamespaces(ctx)
		return NamespacesLoadedMsg{namespaces: namespaces, err: err}
	}
}

func (m *Model) loadKubeConfigs() tea.Cmd {
	return func() tea.Msg {
		configs := m.config.GetRecentKubeConfigs()

		// Add option to enter a new path
		allConfigs := []string{"+ Enter new kubeconfig path..."}

		// Add default kubeconfig
		home, _ := os.UserHomeDir()
		defaultConfig := filepath.Join(home, ".kube", "config")
		allConfigs = append(allConfigs, defaultConfig)

		// Add recent configs (avoiding duplicates)
		for _, cfg := range configs {
			if cfg != defaultConfig {
				allConfigs = append(allConfigs, cfg)
			}
		}

		return KubeConfigsLoadedMsg{configs: allConfigs, err: nil}
	}
}

func (m *Model) loadDeployments() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		deployments, err := m.k8sClient.ListDeployments(ctx, m.namespace)
		return DeploymentsLoadedMsg{deployments: deployments, err: err}
	}
}

func (m *Model) loadPods() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		pods, err := m.k8sClient.ListPodNames(ctx, m.namespace, m.deployment)
		return PodsLoadedMsg{pods: pods, err: err}
	}
}

func (m *Model) loadContainers() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Extract pod name without status
		podName := m.pod
		if idx := strings.Index(podName, " ("); idx != -1 {
			podName = podName[:idx]
		}
		containers, err := m.k8sClient.ListContainers(ctx, m.namespace, podName)
		return ContainersLoadedMsg{containers: containers, err: err}
	}
}

func (m *Model) loadAssetFolders() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		podName := extractPodName(m.pod)
		folders, err := m.k8sClient.ListDirectories(ctx, m.namespace, podName, m.container, "/app/assets")
		return AssetFoldersLoadedMsg{folders: folders, err: err}
	}
}

func (m *Model) executeFastDeploy() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		podName := extractPodName(m.pod)
		localPath := m.inputValue

		// Expand ~ to home directory
		if strings.HasPrefix(localPath, "~/") {
			home, _ := os.UserHomeDir()
			localPath = filepath.Join(home, localPath[2:])
		}

		// Check if local path exists
		info, err := os.Stat(localPath)
		if err != nil {
			return FastDeployCompleteMsg{err: fmt.Errorf("local path error: %w", err)}
		}
		if !info.IsDir() {
			return FastDeployCompleteMsg{err: fmt.Errorf("local path is not a directory: %s", localPath)}
		}

		// Target path is /app/assets/{selected_folder}
		targetPath := fmt.Sprintf("/app/assets/%s", m.assetFolder)

		// Step 1: Clear the target directory
		err = m.k8sClient.ClearDirectory(ctx, m.namespace, podName, m.container, targetPath)
		if err != nil {
			return FastDeployCompleteMsg{err: fmt.Errorf("failed to clear target directory: %w", err)}
		}

		// Step 2: Upload files from local dist to target
		count, err := m.k8sClient.UploadDirectory(ctx, m.namespace, podName, m.container, localPath, targetPath)
		if err != nil {
			return FastDeployCompleteMsg{err: fmt.Errorf("failed to upload files: %w", err)}
		}

		return FastDeployCompleteMsg{result: fmt.Sprintf("Successfully deployed %d files to %s", count, targetPath)}
	}
}

func (m *Model) streamLogs(ctx context.Context, podName string) tea.Cmd {
	return func() tea.Msg {
		// Create a pipe to capture streaming output
		pr, pw := io.Pipe()

		// Start streaming in a goroutine
		go func() {
			defer pw.Close()
			_ = m.k8sClient.StreamLogs(ctx, k8s.LogOptions{
				Namespace:     m.namespace,
				PodName:       podName,
				ContainerName: m.container,
				Follow:        true,
				TailLines:     100,
			}, pw)
		}()

		// Read first line
		reader := bufio.NewReader(pr)
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return LogStreamEndMsg{err: nil}
			}
			return LogStreamEndMsg{err: err}
		}

		return logStreamMsg{
			line:   strings.TrimSuffix(line, "\n"),
			reader: reader,
			pipe:   pr,
		}
	}
}

// logStreamMsg carries streaming state
type logStreamMsg struct {
	line   string
	reader *bufio.Reader
	pipe   *io.PipeReader
}

// readNextLine returns a command that reads the next log line
func readNextLine(reader *bufio.Reader, pipe *io.PipeReader) tea.Cmd {
	return func() tea.Msg {
		line, err := reader.ReadString('\n')
		if err != nil {
			pipe.Close()
			if err == io.EOF {
				return LogStreamEndMsg{err: nil}
			}
			return LogStreamEndMsg{err: err}
		}
		return logStreamMsg{
			line:   strings.TrimSuffix(line, "\n"),
			reader: reader,
			pipe:   pipe,
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logViewer.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle log viewer state separately
		if m.state == StateViewLogs {
			switch msg.String() {
			case "ctrl+c":
				// Cancel streaming if active
				if m.streaming && m.cancelStream != nil {
					m.cancelStream()
					m.streaming = false
				}
				return m, tea.Quit
			case "esc", "q":
				// Cancel streaming if active
				if m.streaming && m.cancelStream != nil {
					m.cancelStream()
					m.streaming = false
				}
				// Save search if there was one
				if m.logViewer.GetSearchQuery() != "" {
					m.config.AddRecentLogSearch(m.logViewer.GetSearchQuery())
				}
				// Go back to command selection
				m.state = StateSelectCommand
				m.cmdSelector.Reset()
				return m, nil
			}
			// Let log viewer handle other keys
			var cmd tea.Cmd
			m.logViewer, cmd = m.logViewer.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "ctrl+n":
			// Switch namespace
			if m.state != StateSelectNamespace {
				m.showNamespaceChange = true
				m.prevStates = append(m.prevStates, m.state)
				m.state = StateSelectNamespace
				m.nsSelector.Reset()
				return m, m.loadNamespaces()
			}

		case "ctrl+k":
			// Switch kubeconfig
			if m.state != StateSelectKubeConfig {
				m.showKubeConfigChange = true
				m.prevStates = append(m.prevStates, m.state)
				m.state = StateSelectKubeConfig
				m.kcSelector.Reset()
				return m, m.loadKubeConfigs()
			}

		case "esc":
			if m.state == StateSelectKubeConfig && m.showKubeConfigChange {
				m.showKubeConfigChange = false
				if len(m.prevStates) > 0 {
					m.state = m.prevStates[len(m.prevStates)-1]
					m.prevStates = m.prevStates[:len(m.prevStates)-1]
				}
				return m, nil
			}
			if m.state == StateSelectNamespace && m.showNamespaceChange {
				m.showNamespaceChange = false
				if len(m.prevStates) > 0 {
					m.state = m.prevStates[len(m.prevStates)-1]
					m.prevStates = m.prevStates[:len(m.prevStates)-1]
				}
				return m, nil
			}
			// Go back to previous state
			return m.goBack()

		case "backspace":
			// Only go back if the text input is empty
			inputEmpty := false
			switch m.state {
			case StateSelectKubeConfig:
				inputEmpty = m.kcSelector.GetInput() == ""
			case StateSelectNamespace:
				inputEmpty = m.nsSelector.GetInput() == ""
			case StateSelectDeployment:
				inputEmpty = m.depSelector.GetInput() == ""
			case StateSelectCommand:
				inputEmpty = m.cmdSelector.GetInput() == ""
			case StateSelectPod:
				inputEmpty = m.podSelector.GetInput() == ""
			case StateSelectContainer:
				inputEmpty = m.contSelector.GetInput() == ""
			case StateInputValue:
				inputEmpty = m.valueInput.Value() == ""
			default:
				inputEmpty = true
			}

			if inputEmpty {
				if m.state == StateSelectKubeConfig && m.showKubeConfigChange {
					m.showKubeConfigChange = false
					if len(m.prevStates) > 0 {
						m.state = m.prevStates[len(m.prevStates)-1]
						m.prevStates = m.prevStates[:len(m.prevStates)-1]
					}
					return m, nil
				}
				if m.state == StateSelectNamespace && m.showNamespaceChange {
					m.showNamespaceChange = false
					if len(m.prevStates) > 0 {
						m.state = m.prevStates[len(m.prevStates)-1]
						m.prevStates = m.prevStates[:len(m.prevStates)-1]
					}
					return m, nil
				}
				return m.goBack()
			}
			// Otherwise, let backspace pass through to the text input

		case "enter":
			return m.handleEnter()

		case "tab":
			return m.handleEnter()
		}

	case NamespacesLoadedMsg:
		if msg.err != nil {
			m.nsSelector.SetError(msg.err)
		} else {
			m.nsSelector.SetItems(msg.namespaces)
		}
		return m, nil

	case KubeConfigsLoadedMsg:
		if msg.err != nil {
			m.kcSelector.SetError(msg.err)
		} else {
			m.kcSelector.SetRecentItems(m.config.GetRecentKubeConfigs())
			m.kcSelector.SetItems(msg.configs)
		}
		return m, nil

	case KubeConfigChangedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = StateShowResult
		} else {
			m.k8sClient = msg.client
			m.kubeconfig = msg.path
			m.config.SetKubeConfig(msg.path)
			m.showKubeConfigChange = false
			// Reset namespace and deployment since we changed cluster
			m.namespace = ""
			m.deployment = ""
			m.state = StateSelectNamespace
			return m, m.loadNamespaces()
		}
		return m, nil

	case DeploymentsLoadedMsg:
		if msg.err != nil {
			m.depSelector.SetError(msg.err)
		} else {
			m.depSelector.SetRecentItems(m.config.GetRecentDeployments(m.namespace))
			m.depSelector.SetItems(msg.deployments)
		}
		return m, nil

	case PodsLoadedMsg:
		if msg.err != nil {
			m.podSelector.SetError(msg.err)
		} else {
			m.podSelector.SetRecentItems(m.config.GetRecentPods(m.deployment))
			m.podSelector.SetItems(msg.pods)
		}
		return m, nil

	case ContainersLoadedMsg:
		if msg.err != nil {
			m.contSelector.SetError(msg.err)
		} else {
			m.contSelector.SetItems(msg.containers)
			// If only one container, auto-select it
			if len(msg.containers) == 1 {
				m.container = msg.containers[0]
				return m.proceedAfterContainer()
			}
		}
		return m, nil

	case CommandResultMsg:
		m.state = StateShowResult
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.result = msg.result
		}
		return m, nil

	case LogsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = StateShowResult
		} else {
			m.logViewer = NewLogViewer()
			m.logViewer.SetSize(m.width, m.height)
			m.logViewer.SetRecentSearches(m.config.GetRecentLogSearches())
			m.logViewer.SetLogs(msg.logs)
			m.logViewer.Focus()
			m.state = StateViewLogs
		}
		return m, nil

	case logStreamMsg:
		// Append the log line and continue reading
		m.logViewer.AppendLog(msg.line)
		return m, readNextLine(msg.reader, msg.pipe)

	case LogStreamEndMsg:
		// Stream ended
		m.streaming = false
		m.logViewer.SetStreaming(false)
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case ExecCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = StateShowResult
		} else {
			return m, tea.Quit
		}
		return m, nil

	case AssetFoldersLoadedMsg:
		if msg.err != nil {
			m.assetSelector.SetError(msg.err)
		} else {
			m.assetSelector.SetItems(msg.folders)
		}
		return m, nil

	case FastDeployCompleteMsg:
		m.state = StateShowResult
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.result = msg.result
		}
		return m, nil
	}

	// Update the active selector
	var cmd tea.Cmd
	switch m.state {
	case StateSelectKubeConfig:
		m.kcSelector, cmd = m.kcSelector.Update(msg)
	case StateSelectNamespace:
		m.nsSelector, cmd = m.nsSelector.Update(msg)
	case StateSelectDeployment:
		m.depSelector, cmd = m.depSelector.Update(msg)
	case StateSelectCommand:
		m.cmdSelector, cmd = m.cmdSelector.Update(msg)
	case StateSelectPod:
		m.podSelector, cmd = m.podSelector.Update(msg)
	case StateSelectContainer:
		m.contSelector, cmd = m.contSelector.Update(msg)
	case StateSelectAssetFolder:
		m.assetSelector, cmd = m.assetSelector.Update(msg)
	case StateInputValue:
		m.valueInput, cmd = m.valueInput.Update(msg)
	}

	return m, cmd
}

func (m Model) goBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateSelectDeployment:
		// Can't go back from deployment if namespace is set
		return m, nil
	case StateSelectCommand:
		m.state = StateSelectDeployment
		m.depSelector.Reset()
		return m, m.loadDeployments()
	case StateSelectPod:
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		return m, nil
	case StateSelectContainer:
		if m.command.NeedsPod {
			m.state = StateSelectPod
			m.podSelector.Reset()
			return m, m.loadPods()
		}
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		return m, nil
	case StateSelectAssetFolder:
		m.state = StateSelectContainer
		m.contSelector.Reset()
		return m, m.loadContainers()
	case StateInputValue:
		// Handle back from fast-deploy input
		if m.command != nil && m.command.Name == "fast-deploy" {
			m.state = StateSelectAssetFolder
			m.assetSelector.Reset()
			return m, m.loadAssetFolders()
		}
		if m.command.NeedsContainer {
			m.state = StateSelectContainer
			m.contSelector.Reset()
			return m, m.loadContainers()
		} else if m.command.NeedsPod {
			m.state = StateSelectPod
			m.podSelector.Reset()
			return m, m.loadPods()
		}
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		return m, nil
	case StateShowResult:
		m.result = ""
		m.err = nil
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		return m, nil
	}
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateSelectKubeConfig:
		selected := m.kcSelector.GetSelected()
		if selected == "" {
			return m, nil
		}

		// Check if user wants to enter a new path
		if strings.HasPrefix(selected, "+ ") {
			// Switch to input mode for new path
			m.valueInput.SetValue("")
			m.valueInput.Placeholder = "Enter kubeconfig path (e.g., ~/.kube/config-prod)"
			m.valueInput.Focus()
			m.state = StateInputValue
			m.command = &Command{Name: "set-kubeconfig", InputPrompt: "Enter kubeconfig file path:"}
			return m, nil
		}

		// Try to create new client with selected config
		return m, func() tea.Msg {
			client, err := k8s.NewClientWithConfig(selected)
			if err != nil {
				return KubeConfigChangedMsg{err: err}
			}
			return KubeConfigChangedMsg{client: client, path: selected}
		}

	case StateSelectNamespace:
		selected := m.nsSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		m.namespace = selected
		m.config.SetNamespace(selected)
		m.showNamespaceChange = false
		m.state = StateSelectDeployment
		m.depSelector.Reset()
		return m, m.loadDeployments()

	case StateSelectDeployment:
		selected := m.depSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		m.deployment = selected
		m.config.AddRecentDeployment(m.namespace, selected)
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		// Set recent commands
		m.cmdSelector.SetRecentItems(m.config.GetRecentCommands())
		return m, nil

	case StateSelectCommand:
		selected := m.cmdSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		// Parse command name from selection
		cmdName := strings.Split(selected, " - ")[0]
		for i := range AvailableCommands {
			if AvailableCommands[i].Name == cmdName {
				m.command = &AvailableCommands[i]
				break
			}
		}
		if m.command == nil {
			return m, nil
		}
		m.config.AddRecentCommand(selected)
		return m.proceedAfterCommand()

	case StateSelectPod:
		selected := m.podSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		m.pod = selected
		m.config.AddRecentPod(m.deployment, selected)
		return m.proceedAfterPod()

	case StateSelectContainer:
		selected := m.contSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		m.container = selected
		return m.proceedAfterContainer()

	case StateSelectAssetFolder:
		selected := m.assetSelector.GetSelected()
		if selected == "" {
			return m, nil
		}
		m.assetFolder = selected
		// Now ask for local dist path
		m.state = StateInputValue
		m.valueInput.SetValue("")
		m.valueInput.Placeholder = "Enter local dist folder path (e.g., ~/project/dist):"
		m.valueInput.Focus()
		return m, nil

	case StateInputValue:
		m.inputValue = m.valueInput.Value()
		if m.inputValue == "" {
			return m, nil
		}

		// Handle kubeconfig path input
		if m.command != nil && m.command.Name == "set-kubeconfig" {
			// Expand ~ to home directory
			path := m.inputValue
			if strings.HasPrefix(path, "~/") {
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, path[2:])
			}
			return m, func() tea.Msg {
				client, err := k8s.NewClientWithConfig(path)
				if err != nil {
					return KubeConfigChangedMsg{err: err}
				}
				return KubeConfigChangedMsg{client: client, path: path}
			}
		}

		// Handle fast-deploy local path input
		if m.command != nil && m.command.Name == "fast-deploy" {
			m.state = StateExecuting
			return m, m.executeFastDeploy()
		}

		return m.executeCommand()

	case StateShowResult:
		m.result = ""
		m.err = nil
		m.state = StateSelectCommand
		m.cmdSelector.Reset()
		return m, nil
	}

	return m, nil
}

func (m Model) proceedAfterCommand() (tea.Model, tea.Cmd) {
	if m.command.NeedsPod {
		m.state = StateSelectPod
		m.podSelector.Reset()
		return m, m.loadPods()
	} else if m.command.NeedsContainer {
		m.state = StateSelectContainer
		m.contSelector.Reset()
		// For container selection without pod, use first pod
		return m, m.loadPodsAndSelectFirst()
	} else if m.command.NeedsInput {
		m.state = StateInputValue
		m.valueInput.SetValue("")
		m.valueInput.Placeholder = m.command.InputPrompt
		m.valueInput.Focus()
		return m, nil
	}
	return m.executeCommand()
}

func (m *Model) loadPodsAndSelectFirst() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		pods, err := m.k8sClient.ListPodNames(ctx, m.namespace, m.deployment)
		if err != nil {
			return PodsLoadedMsg{err: err}
		}
		if len(pods) > 0 {
			m.pod = pods[0]
		}
		containers, err := m.k8sClient.ListContainers(ctx, m.namespace, extractPodName(m.pod))
		return ContainersLoadedMsg{containers: containers, err: err}
	}
}

func extractPodName(podStr string) string {
	if idx := strings.Index(podStr, " ("); idx != -1 {
		return podStr[:idx]
	}
	return podStr
}

// checkShellAvailable checks if a shell is available in the container
func checkShellAvailable(ctx context.Context, client *k8s.Client, namespace, podName, container string) error {
	_, err := client.CheckShellAvailable(ctx, namespace, podName, container)
	return err
}

func (m Model) proceedAfterPod() (tea.Model, tea.Cmd) {
	if m.command.NeedsContainer {
		m.state = StateSelectContainer
		m.contSelector.Reset()
		return m, m.loadContainers()
	} else if m.command.NeedsInput {
		m.state = StateInputValue
		m.valueInput.SetValue("")
		m.valueInput.Placeholder = m.command.InputPrompt
		m.valueInput.Focus()
		return m, nil
	}
	return m.executeCommand()
}

func (m Model) proceedAfterContainer() (tea.Model, tea.Cmd) {
	// Special handling for fast-deploy
	if m.command.Name == "fast-deploy" {
		m.state = StateSelectAssetFolder
		m.assetSelector.Reset()
		return m, m.loadAssetFolders()
	}

	if m.command.NeedsInput {
		m.state = StateInputValue
		m.valueInput.SetValue("")
		m.valueInput.Placeholder = m.command.InputPrompt
		m.valueInput.Focus()
		return m, nil
	}
	return m.executeCommand()
}

func (m Model) executeCommand() (tea.Model, tea.Cmd) {
	m.state = StateExecuting
	ctx := context.Background()
	podName := extractPodName(m.pod)

	switch m.command.Name {
	case "shell":
		// Try to detect if shell is available first
		return m, func() tea.Msg {
			// Try a quick command to check if any shell exists
			err := checkShellAvailable(ctx, m.k8sClient, m.namespace, podName, m.container)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			// Shell is available, exit TUI to run interactive shell
			return ExecCompleteMsg{err: nil}
		}

	case "logs":
		return m, func() tea.Msg {
			logs, err := m.k8sClient.GetLogs(ctx, k8s.LogOptions{
				Namespace:     m.namespace,
				PodName:       podName,
				ContainerName: m.container,
				TailLines:     500,
			})
			return LogsLoadedMsg{logs: logs, err: err}
		}

	case "logs-follow":
		// Start streaming logs
		m.streaming = true
		m.streamCtx, m.cancelStream = context.WithCancel(context.Background())
		m.logViewer = NewLogViewer()
		m.logViewer.SetSize(m.width, m.height)
		m.logViewer.SetRecentSearches(m.config.GetRecentLogSearches())
		m.logViewer.SetLogs("") // Start empty
		m.logViewer.SetStreaming(true)
		m.state = StateViewLogs

		podName := extractPodName(m.pod)
		return m, m.streamLogs(m.streamCtx, podName)

	case "scale":
		replicas, err := strconv.Atoi(m.inputValue)
		if err != nil {
			return m, func() tea.Msg {
				return CommandResultMsg{err: fmt.Errorf("invalid replica count: %s", m.inputValue)}
			}
		}
		return m, func() tea.Msg {
			err := m.k8sClient.ScaleDeployment(ctx, m.namespace, m.deployment, int32(replicas))
			if err != nil {
				return CommandResultMsg{err: err}
			}
			return CommandResultMsg{result: fmt.Sprintf("Scaled %s to %d replicas", m.deployment, replicas)}
		}

	case "update-image":
		return m, func() tea.Msg {
			err := m.k8sClient.UpdateImage(ctx, m.namespace, m.deployment, m.container, m.inputValue)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			return CommandResultMsg{result: fmt.Sprintf("Updated %s image to %s", m.container, m.inputValue)}
		}

	case "port-forward":
		parts := strings.Split(m.inputValue, ":")
		if len(parts) != 2 {
			return m, func() tea.Msg {
				return CommandResultMsg{err: fmt.Errorf("invalid port format, use local:remote")}
			}
		}
		return m, func() tea.Msg {
			return ExecCompleteMsg{err: nil}
		}

	case "rollback":
		revision, err := strconv.ParseInt(m.inputValue, 10, 64)
		if err != nil {
			return m, func() tea.Msg {
				return CommandResultMsg{err: fmt.Errorf("invalid revision number: %s", m.inputValue)}
			}
		}
		return m, func() tea.Msg {
			err := m.k8sClient.RollbackDeployment(ctx, m.namespace, m.deployment, revision)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			return CommandResultMsg{result: fmt.Sprintf("Rolled back %s to revision %d", m.deployment, revision)}
		}

	case "set-env":
		parts := strings.SplitN(m.inputValue, "=", 2)
		if len(parts) != 2 {
			return m, func() tea.Msg {
				return CommandResultMsg{err: fmt.Errorf("invalid format, use KEY=VALUE")}
			}
		}
		return m, func() tea.Msg {
			err := m.k8sClient.SetEnvVar(ctx, m.namespace, m.deployment, m.container, parts[0], parts[1])
			if err != nil {
				return CommandResultMsg{err: err}
			}
			return CommandResultMsg{result: fmt.Sprintf("Set %s=%s on %s", parts[0], parts[1], m.container)}
		}

	case "list-env":
		return m, func() tea.Msg {
			envVars, err := m.k8sClient.GetEnvVars(ctx, m.namespace, m.deployment, m.container)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Environment variables for %s:\n\n", m.container))
			for _, env := range envVars {
				if env.Value != "" {
					result.WriteString(fmt.Sprintf("  %s=%s\n", env.Name, env.Value))
				} else if env.ValueFrom != nil {
					result.WriteString(fmt.Sprintf("  %s=(from secret/configmap)\n", env.Name))
				}
			}
			return CommandResultMsg{result: result.String()}
		}

	case "list-pods":
		return m, func() tea.Msg {
			pods, err := m.k8sClient.ListPods(ctx, m.namespace, m.deployment)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Pods for %s:\n\n", m.deployment))
			for _, pod := range pods {
				status := string(pod.Status.Phase)
				ready := 0
				total := len(pod.Status.ContainerStatuses)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.Ready {
						ready++
					}
				}
				result.WriteString(fmt.Sprintf("  %s  %s  %d/%d\n", pod.Name, status, ready, total))
			}
			return CommandResultMsg{result: result.String()}
		}

	case "list-revisions":
		return m, func() tea.Msg {
			rsList, err := m.k8sClient.GetReplicaSets(ctx, m.namespace, m.deployment)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Revisions for %s:\n\n", m.deployment))
			for _, rs := range rsList {
				revision := rs.Annotations["deployment.kubernetes.io/revision"]
				replicas := *rs.Spec.Replicas
				result.WriteString(fmt.Sprintf("  Revision %s: %d replicas\n", revision, replicas))
			}
			return CommandResultMsg{result: result.String()}
		}

	case "ingress":
		return m, func() tea.Msg {
			ingresses, err := m.k8sClient.GetIngresses(ctx, m.namespace)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Ingresses in %s:\n\n", m.namespace))
			for _, ing := range ingresses {
				result.WriteString(fmt.Sprintf("  %s:\n", ing.Name))
				for _, rule := range ing.Spec.Rules {
					host := rule.Host
					if host == "" {
						host = "*"
					}
					result.WriteString(fmt.Sprintf("    Host: %s\n", host))
					if rule.HTTP != nil {
						for _, path := range rule.HTTP.Paths {
							result.WriteString(fmt.Sprintf("      %s -> %s:%d\n",
								path.Path,
								path.Backend.Service.Name,
								path.Backend.Service.Port.Number))
						}
					}
				}
			}
			return CommandResultMsg{result: result.String()}
		}

	case "describe":
		return m, func() tea.Msg {
			deployment, err := m.k8sClient.GetDeployment(ctx, m.namespace, m.deployment)
			if err != nil {
				return CommandResultMsg{err: err}
			}
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Deployment: %s\n", deployment.Name))
			result.WriteString(fmt.Sprintf("Namespace: %s\n", deployment.Namespace))
			result.WriteString(fmt.Sprintf("Replicas: %d/%d\n", deployment.Status.ReadyReplicas, *deployment.Spec.Replicas))
			result.WriteString(fmt.Sprintf("Strategy: %s\n", deployment.Spec.Strategy.Type))
			result.WriteString("\nContainers:\n")
			for _, container := range deployment.Spec.Template.Spec.Containers {
				result.WriteString(fmt.Sprintf("  %s:\n", container.Name))
				result.WriteString(fmt.Sprintf("    Image: %s\n", container.Image))
				if len(container.Ports) > 0 {
					result.WriteString("    Ports: ")
					for i, port := range container.Ports {
						if i > 0 {
							result.WriteString(", ")
						}
						result.WriteString(fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol))
					}
					result.WriteString("\n")
				}
			}
			return CommandResultMsg{result: result.String()}
		}
	}

	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(RenderHeader(m.kubeconfig, m.namespace, m.deployment))
	b.WriteString("\n")

	// Main content based on state
	switch m.state {
	case StateSelectKubeConfig:
		if m.k8sClient == nil && m.initialClientErr != nil {
			b.WriteString(WarningStyle.Render("No kubeconfig found or configured."))
			b.WriteString("\n")
			b.WriteString(InfoStyle.Render("Please select or enter a kubeconfig path:"))
			b.WriteString("\n\n")
		} else if m.showKubeConfigChange {
			b.WriteString(InfoStyle.Render("Changing kubeconfig..."))
			b.WriteString("\n\n")
		}
		b.WriteString(m.kcSelector.View())

	case StateSelectNamespace:
		if m.showNamespaceChange {
			b.WriteString(InfoStyle.Render("Changing namespace..."))
			b.WriteString("\n\n")
		}
		b.WriteString(m.nsSelector.View())

	case StateSelectDeployment:
		b.WriteString(m.depSelector.View())

	case StateSelectCommand:
		b.WriteString(m.cmdSelector.View())

	case StateSelectPod:
		b.WriteString(m.podSelector.View())

	case StateSelectContainer:
		b.WriteString(m.contSelector.View())

	case StateSelectAssetFolder:
		b.WriteString(InfoStyle.Render("Select asset folder to deploy to:"))
		b.WriteString("\n\n")
		b.WriteString(m.assetSelector.View())

	case StateInputValue:
		if m.command != nil && m.command.Name == "fast-deploy" {
			b.WriteString(InfoStyle.Render(fmt.Sprintf("Target: /app/assets/%s", m.assetFolder)))
			b.WriteString("\n\n")
			b.WriteString(LabelStyle.Render("Enter local dist folder path:"))
		} else {
			b.WriteString(LabelStyle.Render(m.command.InputPrompt))
		}
		b.WriteString("\n")
		b.WriteString(FocusedInputStyle.Render(m.valueInput.View()))

	case StateExecuting:
		b.WriteString(RenderLoading("Executing command..."))

	case StateShowResult:
		if m.err != nil {
			b.WriteString(RenderError(m.err.Error()))
		} else {
			b.WriteString(SuccessStyle.Render("Result:"))
			b.WriteString("\n\n")
			b.WriteString(m.result)
		}
		b.WriteString("\n\n")
		b.WriteString(InfoStyle.Render("Press Enter to continue..."))

	case StateViewLogs:
		// Skip the header for log viewer to maximize space
		var logView strings.Builder
		logView.WriteString(m.logViewer.View())
		logView.WriteString("\n")
		help := []string{"Tab: toggle search", "↑↓: scroll (when not typing)", "PgUp/PgDn: page", "Enter: exit search", "Ctrl+L: clear", "Esc/q: back"}
		logView.WriteString(RenderHelp(help...))
		return lipgloss.NewStyle().Padding(1, 2).Render(logView.String())
	}

	// Help
	b.WriteString("\n\n")
	help := []string{"↑↓: navigate", "Enter: select", "Esc/Backspace: back", "Ctrl+K: kubeconfig", "Ctrl+N: namespace", "Ctrl+C: quit"}
	b.WriteString(RenderHelp(help...))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// RunShell runs an interactive shell after exiting bubble tea
func RunShell(k8sClient *k8s.Client, namespace, pod, container, shell string) error {
	ctx := context.Background()
	podName := extractPodName(pod)
	return k8sClient.Shell(ctx, namespace, podName, container, shell)
}

// RunLogs streams logs after exiting bubble tea
func RunLogs(k8sClient *k8s.Client, namespace, pod, container string, follow bool) error {
	ctx := context.Background()
	podName := extractPodName(pod)
	tailLines := int64(100)
	return k8sClient.StreamLogs(ctx, k8s.LogOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Follow:        follow,
		TailLines:     tailLines,
	}, os.Stdout)
}

// RunPortForward runs port forwarding after exiting bubble tea
func RunPortForward(k8sClient *k8s.Client, namespace, pod string, localPort, remotePort int) error {
	ctx := context.Background()
	podName := extractPodName(pod)
	return k8sClient.PortForward(ctx, k8s.PortForwardOptions{
		Namespace:  namespace,
		PodName:    podName,
		LocalPort:  localPort,
		RemotePort: remotePort,
	})
}

// Getter methods for accessing model state after TUI exits
func (m Model) GetNamespace() string {
	return m.namespace
}

func (m Model) GetDeployment() string {
	return m.deployment
}

func (m Model) GetCommand() *Command {
	return m.command
}

func (m Model) GetPod() string {
	return m.pod
}

func (m Model) GetContainer() string {
	return m.container
}

func (m Model) GetInputValue() string {
	return m.inputValue
}
