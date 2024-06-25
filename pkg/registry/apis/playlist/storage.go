package playlist

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"

	playlist "github.com/grafana/grafana/pkg/apis/playlist/v0alpha1"
	grafanaregistry "github.com/grafana/grafana/pkg/apiserver/registry/generic"
	grafanarest "github.com/grafana/grafana/pkg/apiserver/rest"
)

var _ grafanarest.Storage = (*storage)(nil)

type storage struct {
	*genericregistry.Store
}

func newStorage(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter, legacy *legacyStorage) (*storage, error) {
	strategy := grafanaregistry.NewStrategy(scheme)

	resource := playlist.PlaylistResourceInfo
	store := &genericregistry.Store{
		NewFunc:                   resource.NewFunc,
		NewListFunc:               resource.NewListFunc,
		KeyRootFunc:               grafanaregistry.KeyRootFunc(resourceInfo.GroupResource()),
		KeyFunc:                   grafanaregistry.NamespaceKeyFunc(resourceInfo.GroupResource()),
		PredicateFunc:             grafanaregistry.Matcher,
		DefaultQualifiedResource:  resource.GroupResource(),
		SingularQualifiedResource: resourceInfo.SingularGroupResource(),
		TableConvertor:            legacy.tableConverter,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: grafanaregistry.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}
	return &storage{Store: store}, nil
}

// Compare asserts on the equality of objects returned from both stores	(object storage and legacy storage)
func (s *storage) Compare(storageObj, legacyObj runtime.Object) bool {
	accStr, err := meta.Accessor(storageObj)
	if err != nil {
		return false
	}
	accLegacy, err := meta.Accessor(legacyObj)
	if err != nil {
		return false
	}

	return accStr.GetName() == accLegacy.GetName()
}
