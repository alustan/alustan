package listers

import (
    "errors"

    "github.com/alustan/api/v1alpha1"
    "k8s.io/client-go/tools/cache"
    "k8s.io/apimachinery/pkg/labels"
)

type TerraformLister interface {
    List(selector labels.Selector) (ret []*v1alpha1.Terraform, err error)
    Terraform(namespace string) TerraformNamespaceLister
}

type terraformLister struct {
    indexer cache.Indexer
}

func NewTerraformLister(indexer cache.Indexer) TerraformLister {
    return &terraformLister{indexer: indexer}
}

func (l *terraformLister) List(selector labels.Selector) (ret []*v1alpha1.Terraform, err error) {
    err = cache.ListAll(l.indexer, selector, func(m interface{}) {
        ret = append(ret, m.(*v1alpha1.Terraform))
    })
    return ret, err
}

func (l *terraformLister) Terraform(namespace string) TerraformNamespaceLister {
    return terraformNamespaceLister{indexer: l.indexer, namespace: namespace}
}

type TerraformNamespaceLister interface {
    List(selector labels.Selector) (ret []*v1alpha1.Terraform, err error)
    Get(name string) (*v1alpha1.Terraform, error)
}

type terraformNamespaceLister struct {
    indexer   cache.Indexer
    namespace string
}

func (l terraformNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Terraform, err error) {
    err = cache.ListAllByNamespace(l.indexer, l.namespace, selector, func(m interface{}) {
        ret = append(ret, m.(*v1alpha1.Terraform))
    })
    return ret, err
}

func (l terraformNamespaceLister) Get(name string) (*v1alpha1.Terraform, error) {
    obj, exists, err := l.indexer.GetByKey(l.namespace + "/" + name)
    if !exists {
        return nil, errors.New("terraform resource not found")
    }
    return obj.(*v1alpha1.Terraform), err
}
