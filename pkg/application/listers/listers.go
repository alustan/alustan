package listers

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type AppLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	GetByKey(key string) (runtime.Object, error)
	App(namespace string) AppNamespaceLister
}

type appLister struct {
	indexer cache.Indexer
}

func NewAppLister(indexer cache.Indexer) AppLister {
	return &appLister{indexer: indexer}
}

func (l *appLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAll(l.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *appLister) GetByKey(key string) (runtime.Object, error) {
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("app resource not found")
	}
	return obj.(runtime.Object), nil
}

func (l *appLister) App(namespace string) AppNamespaceLister {
	return &appNamespaceLister{
		indexer:   l.indexer,
		namespace: namespace,
	}
}

type AppNamespaceLister interface {
	List(selector labels.Selector) ([]runtime.Object, error)
	Get(name string) (runtime.Object, error)
}

type appNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

func (l *appNamespaceLister) List(selector labels.Selector) ([]runtime.Object, error) {
	var ret []runtime.Object
	err := cache.ListAllByNamespace(l.indexer, l.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(runtime.Object))
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (l *appNamespaceLister) Get(name string) (runtime.Object, error) {
	key := l.namespace + "/" + name
	obj, exists, err := l.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("app resource not found")
	}
	return obj.(runtime.Object), nil
}
