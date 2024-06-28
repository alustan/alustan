package pluginregistry

import (
	"errors"
    "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// Plugin interface
type Plugin interface {
	CreateApplicationSet() v1alpha1.ApplicationSet
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


