package pluginregistry

import (
	"github.com/alustan/pkg/app-controller/pluginregistry/aws"
	
)



func SetupPlugin(providerType, strategy, name, cluster, namespace, image, port,subDomain, gitOwner, gitRepo, configData string) (Plugin, error) {
	switch providerType {
	case "aws":
		applicationSetPlugin := aws.NewApplicationSetPlugin(strategy, name, cluster, namespace, image, port,subDomain, gitOwner, gitRepo, configData)
		RegisterPlugin("aws", applicationSetPlugin)
		return applicationSetPlugin, nil
	default:
		plugin, err := GetPlugin(providerType)
		if err != nil {
			return nil, err
		}
		return plugin, nil
	}
}
