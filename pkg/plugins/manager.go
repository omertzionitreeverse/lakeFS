package plugins

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/treeverse/lakefs/pkg/logging"
)

var (
	ErrPluginTypeNotFound   = errors.New("unknown plugin type")
	ErrPluginNameNotFound   = errors.New("unknown plugin name")
	ErrUninitializedManager = errors.New("uninitialized plugins manager")
)

var allowedProtocols = []plugin.Protocol{
	plugin.ProtocolGRPC,
}

type PluginType int

const (
	Diff PluginType = iota
)

type ClosingFunc func()

// PluginIdentity identifies the plugin's version and executable location.
type PluginIdentity struct {
	Version            int
	ExecutableLocation string
}

// PluginAuth includes authentication properties for the plugin.
type PluginAuth struct {
	Key   string
	Value string
}

// The Manager holds different maps for different kinds of possible plugin clients.
// For example, the diffClients map might contain a mapping of "delta" -> plugin.Client to communicate with the Delta
// plugin.
type Manager struct {
	diffClients map[string]*plugin.Client
}

func NewManager() *Manager {
	return &Manager{
		diffClients: make(map[string]*plugin.Client),
	}
}

// RegisterPlugin is used to register a new plugin client with the corresponding plugin type.
func (m *Manager) RegisterPlugin(name string, id PluginIdentity, auth PluginAuth, pt PluginType, p plugin.Plugin) error {
	if m == nil {
		return ErrUninitializedManager
	}
	switch pt {
	case Diff:
		return m.registerDiffPlugin(name, id, auth, p)
	default:
		return ErrPluginTypeNotFound
	}
}

func (m *Manager) registerDiffPlugin(name string, id PluginIdentity, auth PluginAuth, p plugin.Plugin) error {
	hc := plugin.HandshakeConfig{
		ProtocolVersion:  uint(id.Version),
		MagicCookieKey:   auth.Key,
		MagicCookieValue: auth.Value,
	}
	cmd := exec.Command(id.ExecutableLocation) //nolint:gosec
	c, err := pluginClient(name, p, hc, cmd)
	if err != nil {
		return err
	}
	m.diffClients[name] = c
	return nil
}

func pluginClient(name string, p plugin.Plugin, hc plugin.HandshakeConfig, cmd *exec.Cmd) (*plugin.Client, error) {
	clientConfig := plugin.ClientConfig{
		Plugins: map[string]plugin.Plugin{
			name: p,
		},
		AllowedProtocols: allowedProtocols,
		HandshakeConfig:  hc,
		Cmd:              cmd,
	}
	return newPluginClient(name, clientConfig)
}

func newPluginClient(clientName string, clientConfig plugin.ClientConfig) (*plugin.Client, error) {
	hl := hclog.New(&hclog.LoggerOptions{
		Name:   fmt.Sprintf("%s_logger", clientName),
		Output: os.Stdout,
		Level:  hclog.Debug,
	})
	l := logging.Default()
	hcl := NewHClogger(hl, l)
	clientConfig.Logger = hcl
	return plugin.NewClient(&clientConfig), nil
}

// LoadDiffPluginClient initializes a plugin.Client, and returns a Differ client and a ClosingFunc to Kill() the client
// after being used.
func (m *Manager) LoadDiffPluginClient(name string) (interface{}, ClosingFunc, error) {
	if m == nil {
		return nil, nil, ErrUninitializedManager
	}
	c, ok := m.diffClients[name]
	if !ok {
		return nil, nil, ErrPluginNameNotFound
	}
	grpcClient, err := c.Client()
	if err != nil {
		return nil, nil, err
	}
	stub, err := grpcClient.Dispense(name)
	if err != nil {
		return nil, nil, err
	}
	return stub, func() { c.Kill() }, nil
}
