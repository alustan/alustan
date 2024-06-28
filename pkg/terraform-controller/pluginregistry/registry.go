package pluginregistry

import (
	"errors"
	
)

// Plugin interface
type Plugin interface {
	GetDockerfileAdditions() string
	Execute() (map[string]interface{}, error)
}

var plugins = make(map[string]Plugin)

func RegisterPlugin(providerType string, plugin Plugin) {
	plugins[providerType] = plugin
}

func GetPlugin(providerType string) (Plugin, error) {
	plugin, exists := plugins[providerType]
	if !exists {
		return nil, errors.New("unknown plugin type")
	}
	return plugin, nil
}


