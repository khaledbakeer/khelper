# khelper - Interactive Kubernetes Deployment Helper

A modern, interactive CLI tool for Kubernetes deployment management with a beautiful terminal UI.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## Features

- 🎨 **Modern Terminal UI** - Built with Charmbracelet's Bubble Tea, Bubbles, and Lip Gloss
- 🔍 **Fuzzy Search** - Real-time filtering as you type in all selection lists
- ⌨️ **Keyboard Navigation** - Full keyboard support (↑↓, Tab, Enter, Esc, Backspace)
- 💾 **Persistent Config** - Remembers last namespace, kubeconfig, recent deployments, pods, and commands
- 🔄 **Recent Items** - Quick access to recently used items at the top of each list
- 📋 **In-App Log Viewer** - View and search logs without leaving the TUI
- 🔴 **Streaming Logs** - Real-time log following with search capability
- 🔀 **Multi-Kubeconfig** - Switch between different kubeconfig files with Ctrl+K
- 🐚 **Smart Shell Detection** - Auto-detects available shell (bash/sh/ash)
- 🚀 **Fast Deploy** - Upload local dist folder directly to container

## Installation

### From Source

\`\`\`bash
# Clone the repository
git clone https://github.com/khaledbakeer/khelper.git
cd khelper

# Build
go build -o khelper ./cmd/khelper/

# Install (optional)
sudo mv khelper /usr/local/bin/
\`\`\`

### Using Go Install

\`\`\`bash
go install github.com/khaledbakeer/khelper/cmd/khelper@latest
\`\`\`

## Usage

### Interactive Mode

Simply run \`khelper\` without arguments to start the interactive wizard:

\`\`\`bash
khelper
\`\`\`

The tool will guide you through:
1. **Namespace Selection** - Pick from available namespaces (saved for next time)
2. **Deployment Selection** - Choose a deployment with fuzzy search
3. **Command Selection** - Select an action to perform
4. **Pod/Container Selection** - If needed, select specific pod and container
5. **Execute** - Run the command with visual feedback

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| ↑/↓ | Navigate list |
| Enter/Tab | Select item |
| Esc/Backspace | Go back to previous step |
| Ctrl+K | Change kubeconfig |
| Ctrl+N | Change namespace |
| Ctrl+C | Quit |

### Log Viewer Shortcuts

| Key | Action |
|-----|--------|
| Tab | Toggle search mode |
| ↑/↓ | Scroll logs / Navigate search results |
| PgUp/PgDn | Page up/down |
| Enter | View full log entry / Exit search |
| Ctrl+L | Clear search |
| Esc/q | Exit log viewer |

### Available Commands

| Command | Description |
|---------|-------------|
| \`logs\` | View container logs in TUI with search |
| \`logs-follow\` | Stream container logs in real-time |
| \`shell\` | Open interactive shell (auto-detects bash/sh/ash) |
| \`fast-deploy\` | Upload local dist folder to /app/assets |
| \`scale\` | Scale deployment replicas |
| \`update-image\` | Update container image |
| \`port-forward\` | Forward local port to pod |
| \`rollback\` | Rollback to previous revision |
| \`set-env\` | Set environment variable |
| \`list-env\` | List environment variables |
| \`list-pods\` | List all pods in deployment |
| \`list-revisions\` | List deployment revisions |
| \`ingress\` | Show related ingresses |
| \`describe\` | Show deployment details |

## Configuration

Configuration is stored in \`~/.khelper/config.yml\`:

\`\`\`yaml
last_namespace: production
kubeconfig: /home/user/.kube/config-prod
recent_kubeconfigs:
  - /home/user/.kube/config
  - /home/user/.kube/config-prod
recent_deployments:
  production:
    - my-app
    - api-server
recent_pods:
  my-app:
    - my-app-7d8f9c6b5d-abc12
recent_commands:
  - logs - View container logs
  - shell - Open shell
recent_log_searches:
  - error
  - exception
\`\`\`

## Requirements

- Go 1.21+
- Valid Kubernetes configuration (\`~/.kube/config\` or \`KUBECONFIG\` env var)
- Cluster access with appropriate permissions

## Tech Stack

- **[Cobra](https://github.com/spf13/cobra)** - CLI command structure
- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** - Terminal UI framework
- **[Bubbles](https://github.com/charmbracelet/bubbles)** - TUI components
- **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** - Style definitions
- **[client-go](https://github.com/kubernetes/client-go)** - Kubernetes API client
- **[sahilm/fuzzy](https://github.com/sahilm/fuzzy)** - Fuzzy search

## License

MIT License
