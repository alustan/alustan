package aws

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationSetPlugin struct {
	strategy string
	name string
	cluster string
	namespace string
	image string
	port string
	subDomain string
	gitOwner string
	gitRepo string
	configData string
}

func NewApplicationSetPlugin(strategy, name, cluster, namespace, image, port,subDomain, gitOwner, gitRepo, configData string) *ApplicationSetPlugin {
	return &ApplicationSetPlugin{
		strategy: strategy,
		name:    name,
		cluster:  cluster,
	    namespace: namespace,
	    image: image,
	    port: port  ,
	    subDomain: subDomain,
	    gitOwner: gitOwner,
	    gitRepo: gitRepo,
	   configData: configData,
	}
}

func (p *ApplicationSetPlugin) CreateApplicationSet() v1alpha1.ApplicationSet {
	
	desiredResources := ApplicationSet(p.strategy, p.name, p.cluster, p.namespace, p.image, p.port, p.subDomain, p.gitOwner, p.gitRepo, p.configData)
   
	return desiredResources
}


func ApplicationSet(strategy, name, cluster, namespace, image, port, subDomain, gitOwner, gitRepo, configData string) v1alpha1.ApplicationSet {
	var generators []v1alpha1.ApplicationSetGenerator
	var workloadType string
	var backendServiceName string
	var backendServicePort string

	// Determine the workload type and backend service based on the strategy
	if strategy == "canary" {
		workloadType = "rollout"
		backendServiceName = fmt.Sprintf("%s-root", name)
		backendServicePort = "use-annotation"
	} else {
		workloadType = "deployment"
		backendServiceName = name
		backendServicePort = "http"
	}

	// Define generators based on the strategy
	if strategy == "preview" {
		generators = []v1alpha1.ApplicationSetGenerator{
			{
				Matrix: &v1alpha1.MatrixGenerator{
					Generators: []v1alpha1.ApplicationSetNestedGenerator{
						{
							PullRequest: &v1alpha1.PullRequestGenerator{
								Github: &v1alpha1.PullRequestGeneratorGithub{
									Owner:  gitOwner,
									Repo:   gitRepo,
									Labels: []string{"preview"},
								},
							},
						},
						{
							Clusters: &v1alpha1.ClusterGenerator{
								Selector: metav1.LabelSelector{
									MatchLabels: map[string]string{
										"environment": cluster,
									},
								},
							},
						},
					},
				},
			}, 
		}
	} else {
		generators = []v1alpha1.ApplicationSetGenerator{
			{
				Clusters: &v1alpha1.ClusterGenerator{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"environment": cluster,
						},
					},
				},
			}, 
		}
	}

	return v1alpha1.ApplicationSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "ApplicationSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			SyncPolicy: &v1alpha1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: false,
			},
			GoTemplate:        true,
			GoTemplateOptions: []string{"missingkey=error"},
			Generators:        generators,
			Template: v1alpha1.ApplicationSetTemplate{
				ApplicationSetTemplateMeta: v1alpha1.ApplicationSetTemplateMeta{
					Name: name,
					Labels: map[string]string{
						"workload": "true",
					},
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Destination: v1alpha1.ApplicationDestination{
						Name:      "{{.name}}",
						Namespace: namespace,
					},
					SyncPolicy: &v1alpha1.SyncPolicy{
						Automated: &v1alpha1.SyncPolicyAutomated{},
						SyncOptions: []string{
							"CreateNamespace=true",
						},
					},
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "{{.metadata.annotations.addons_repo_url}}",
						Path:           "aws-app",
						TargetRevision: "{{.metadata.annotations.workload_repo_revision}}",
						Helm: &v1alpha1.ApplicationSourceHelm{
							ReleaseName: "app",
							Values: fmt.Sprintf(`
image:
  repository: %s
nameOverride: %s
fullnameOverride: %s
service:
  port: %s
namespace: %s
prometheus:
  name: %s-prom
externalSecret:
  region: {{.metadata.annotations.aws_region}}
  key: {{.metadata.annotations.secret_store}}
ingress:
  annotations:
    alb.ingress.kubernetes.io/certificate-arn: {{.metadata.annotations.aws_certificate_arn}}
  hosts:
  - host: %s
    paths:
      backendServiceName: %s
      backendServicePort: %s
  tls:
  - hosts:
    - %s
strategy:
  canary:
    enabled: %t
    canaryService: %s-canary
    stableService: %s-stable
    rootService:  %s-root
externalNameService:
  dbAlias: {{.metadata.annotations.db_engine}}
  externalName: {{.metadata.annotations.db_instance_address}}
rolloutServices:
  - name: %s-root
  - name: %s-canary
  - name: %s-stable
config:
  data:
%s
workload:
  type: %s
`, image, name, name, port, namespace, name, subDomain, backendServiceName, backendServicePort, subDomain, strategy == "canary", name, name, name, name, name, name, configData, workloadType),
						},
					},
				},
			},
		},
	}
}
