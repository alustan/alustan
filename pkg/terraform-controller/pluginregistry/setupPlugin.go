package pluginregistry

import (
	"github.com/alustan/pkg/terraform-controller/pluginregistry/aws"
)



func SetupPlugin(providerType, workspace, region string) (Plugin, error) {
	switch providerType {
	case "aws":
		awsPlugin := aws.NewAWSPlugin( workspace, region)
		RegisterPlugin("aws", awsPlugin)
		return awsPlugin, nil
	default:
		plugin, err := GetPlugin(providerType)
		if err != nil {
			return nil, err
		}
		return plugin, nil
	}
}
