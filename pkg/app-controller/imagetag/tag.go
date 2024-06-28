package imagetag

type RegistryClientInterface interface {
	GetTags(imageName string) ([]string, error)
}