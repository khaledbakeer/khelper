package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const MaxRecentItems = 5

type Config struct {
	LastNamespace     string              `yaml:"last_namespace"`
	KubeConfig        string              `yaml:"kubeconfig,omitempty"`
	RecentKubeConfigs []string            `yaml:"recent_kubeconfigs,omitempty"`
	RecentDeployments map[string][]string `yaml:"recent_deployments,omitempty"` // namespace -> deployments
	RecentCommands    []string            `yaml:"recent_commands,omitempty"`
	RecentPods        map[string][]string `yaml:"recent_pods,omitempty"` // deployment -> pods
	RecentLogSearches []string            `yaml:"recent_log_searches,omitempty"`
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".khelper", "config.yml"), nil
}

func Load() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		RecentDeployments: make(map[string][]string),
		RecentPods:        make(map[string][]string),
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Initialize maps if nil
	if cfg.RecentDeployments == nil {
		cfg.RecentDeployments = make(map[string][]string)
	}
	if cfg.RecentPods == nil {
		cfg.RecentPods = make(map[string][]string)
	}

	return cfg, nil
}

func (c *Config) Save() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func (c *Config) SetNamespace(ns string) error {
	c.LastNamespace = ns
	return c.Save()
}

// addToRecent adds an item to the front of a recent list, removing duplicates
func addToRecent(list []string, item string) []string {
	// Remove existing occurrence
	newList := make([]string, 0, MaxRecentItems)
	for _, existing := range list {
		if existing != item {
			newList = append(newList, existing)
		}
	}
	// Add to front
	newList = append([]string{item}, newList...)
	// Limit size
	if len(newList) > MaxRecentItems {
		newList = newList[:MaxRecentItems]
	}
	return newList
}

// AddRecentDeployment adds a deployment to recent list for a namespace
func (c *Config) AddRecentDeployment(namespace, deployment string) error {
	c.RecentDeployments[namespace] = addToRecent(c.RecentDeployments[namespace], deployment)
	return c.Save()
}

// GetRecentDeployments returns recent deployments for a namespace
func (c *Config) GetRecentDeployments(namespace string) []string {
	return c.RecentDeployments[namespace]
}

// AddRecentCommand adds a command to recent list
func (c *Config) AddRecentCommand(command string) error {
	c.RecentCommands = addToRecent(c.RecentCommands, command)
	return c.Save()
}

// GetRecentCommands returns recent commands
func (c *Config) GetRecentCommands() []string {
	return c.RecentCommands
}

// AddRecentPod adds a pod to recent list for a deployment
func (c *Config) AddRecentPod(deployment, pod string) error {
	c.RecentPods[deployment] = addToRecent(c.RecentPods[deployment], pod)
	return c.Save()
}

// GetRecentPods returns recent pods for a deployment
func (c *Config) GetRecentPods(deployment string) []string {
	return c.RecentPods[deployment]
}

// AddRecentLogSearch adds a log search term to recent list
func (c *Config) AddRecentLogSearch(search string) error {
	if search == "" {
		return nil
	}
	c.RecentLogSearches = addToRecent(c.RecentLogSearches, search)
	return c.Save()
}

// GetRecentLogSearches returns recent log searches
func (c *Config) GetRecentLogSearches() []string {
	return c.RecentLogSearches
}

// SetKubeConfig sets the kubeconfig path
func (c *Config) SetKubeConfig(path string) error {
	c.KubeConfig = path
	c.RecentKubeConfigs = addToRecent(c.RecentKubeConfigs, path)
	return c.Save()
}

// GetKubeConfig returns the kubeconfig path
func (c *Config) GetKubeConfig() string {
	return c.KubeConfig
}

// GetRecentKubeConfigs returns recent kubeconfig paths
func (c *Config) GetRecentKubeConfigs() []string {
	return c.RecentKubeConfigs
}

// AddRecentKubeConfig adds a kubeconfig to recent list
func (c *Config) AddRecentKubeConfig(path string) error {
	c.RecentKubeConfigs = addToRecent(c.RecentKubeConfigs, path)
	return c.Save()
}
