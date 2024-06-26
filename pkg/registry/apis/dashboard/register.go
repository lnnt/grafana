package dashboard

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	common "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/spec3"

	"github.com/grafana/grafana/pkg/apis/dashboard/v0alpha1"
	dashboard "github.com/grafana/grafana/pkg/apis/dashboard/v0alpha1"
	"github.com/grafana/grafana/pkg/apiserver/builder"
	grafanarest "github.com/grafana/grafana/pkg/apiserver/rest"
	"github.com/grafana/grafana/pkg/infra/db"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/infra/tracing"
	"github.com/grafana/grafana/pkg/registry/apis/dashboard/access"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/apiserver/endpoints/request"
	gapiutil "github.com/grafana/grafana/pkg/services/apiserver/utils"
	"github.com/grafana/grafana/pkg/services/dashboards"
	dashver "github.com/grafana/grafana/pkg/services/dashboardversion"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/provisioning"
	"github.com/grafana/grafana/pkg/setting"
)

var _ builder.APIGroupBuilder = (*DashboardsAPIBuilder)(nil)

// This is used just so wire has something unique to return
type DashboardsAPIBuilder struct {
	dashboardService dashboards.DashboardService

	accessControl accesscontrol.AccessControl
	store         *dashboardStorage

	log log.Logger
}

func RegisterAPIService(cfg *setting.Cfg, features featuremgmt.FeatureToggles,
	apiregistration builder.APIRegistrar,
	dashboardService dashboards.DashboardService,
	dashboardVersionService dashver.Service,
	accessControl accesscontrol.AccessControl,
	provisioning provisioning.ProvisioningService,
	dashStore dashboards.Store,
	reg prometheus.Registerer,
	sql db.DB,
	tracing *tracing.TracingService,
) *DashboardsAPIBuilder {
	if !features.IsEnabledGlobally(featuremgmt.FlagGrafanaAPIServerWithExperimentalAPIs) {
		return nil // skip registration unless opting into experimental apis
	}

	namespacer := request.GetNamespaceMapper(cfg)
	builder := &DashboardsAPIBuilder{
		log: log.New("grafana-apiserver.dashboards"),

		dashboardService: dashboardService,
		accessControl:    accessControl,

		store: &dashboardStorage{
			resource: dashboard.DashboardResourceInfo,
			access:   access.NewDashboardAccess(sql, namespacer, dashStore, provisioning, dashboardVersionService),
			tableConverter: gapiutil.NewTableConverter(
				dashboard.DashboardResourceInfo.GroupResource(),
				[]metav1.TableColumnDefinition{
					{Name: "Name", Type: "string", Format: "name"},
					{Name: "Title", Type: "string", Format: "string", Description: "The dashboard name"},
					{Name: "Created At", Type: "date"},
				},
				func(obj any) ([]interface{}, error) {
					dash, ok := obj.(*v0alpha1.Dashboard)
					if ok {
						if dash != nil {
							return []interface{}{
								dash.Name,
								dash.Spec.GetNestedString("title"),
								dash.CreationTimestamp.UTC().Format(time.RFC3339),
							}, nil
						}
					}
					return nil, fmt.Errorf("expected dashboard or summary")
				}),
		},
	}
	apiregistration.RegisterAPI(builder)
	return builder
}

func (b *DashboardsAPIBuilder) GetGroupVersion() schema.GroupVersion {
	return v0alpha1.DashboardResourceInfo.GroupVersion()
}

func (b *DashboardsAPIBuilder) GetDesiredDualWriterMode(dualWrite bool, modeMap map[string]grafanarest.DualWriterMode) grafanarest.DualWriterMode {
	// Add required configuration support in order to enable other modes. For an example, see pkg/registry/apis/playlist/register.go
	return grafanarest.Mode0
}

func addKnownTypes(scheme *runtime.Scheme, gv schema.GroupVersion) {
	scheme.AddKnownTypes(gv,
		&v0alpha1.Dashboard{},
		&v0alpha1.DashboardList{},
		&v0alpha1.DashboardWithAccessInfo{},
		&v0alpha1.DashboardVersionList{},
		&v0alpha1.VersionsQueryOptions{},
		&metav1.PartialObjectMetadata{},
		&metav1.PartialObjectMetadataList{},
	)
}

func (b *DashboardsAPIBuilder) InstallSchema(scheme *runtime.Scheme) error {
	resourceInfo := v0alpha1.DashboardResourceInfo
	addKnownTypes(scheme, resourceInfo.GroupVersion())

	// Link this version to the internal representation.
	// This is used for server-side-apply (PATCH), and avoids the error:
	//   "no kind is registered for the type"
	addKnownTypes(scheme, schema.GroupVersion{
		Group:   resourceInfo.GroupVersion().Group,
		Version: runtime.APIVersionInternal,
	})

	// If multiple versions exist, then register conversions from zz_generated.conversion.go
	// if err := playlist.RegisterConversions(scheme); err != nil {
	//   return err
	// }
	metav1.AddToGroupVersion(scheme, resourceInfo.GroupVersion())
	return scheme.SetVersionPriority(resourceInfo.GroupVersion())
}

func (b *DashboardsAPIBuilder) GetAPIGroupInfo(
	scheme *runtime.Scheme,
	codecs serializer.CodecFactory, // pointer?
	optsGetter generic.RESTOptionsGetter,
	desiredMode grafanarest.DualWriterMode,
	reg prometheus.Registerer,
) (*genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(v0alpha1.GROUP, scheme, metav1.ParameterCodec, codecs)

	dash := b.store.resource
	legacyStore, err := b.store.newStore(scheme, optsGetter)
	if err != nil {
		return nil, err
	}

	storage := map[string]rest.Storage{}
	storage[dash.StoragePath()] = legacyStore
	storage[dash.StoragePath("dto")] = &DTOConnector{
		builder: b,
	}
	storage[dash.StoragePath("versions")] = &VersionsREST{
		search: b.store.server, // resource.NewLocalResourceSearchClient(b.store.server),
	}

	// // Dual writes if a RESTOptionsGetter is provided
	// if desiredMode != grafanarest.Mode0 && optsGetter != nil {
	// 	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: grafanaregistry.GetAttrs}
	// 	if err := store.CompleteWithOptions(options); err != nil {
	// 		return nil, err
	// 	}
	// 	storage[resourceInfo.StoragePath()] = grafanarest.NewDualWriter(
	// 		grafanarest.Mode1,
	// 		store, //legacyStore,
	// 		store,
	// 		reg)
	// }

	apiGroupInfo.VersionedResourcesStorageMap[v0alpha1.VERSION] = storage
	return &apiGroupInfo, nil
}

func (b *DashboardsAPIBuilder) GetOpenAPIDefinitions() common.GetOpenAPIDefinitions {
	return v0alpha1.GetOpenAPIDefinitions
}

func (b *DashboardsAPIBuilder) PostProcessOpenAPI(oas *spec3.OpenAPI) (*spec3.OpenAPI, error) {
	// The plugin description
	oas.Info.Description = "Grafana dashboards as resources"

	// The root api URL
	root := "/apis/" + b.GetGroupVersion().String() + "/"

	// Hide the ability to list or watch across all tenants
	delete(oas.Paths.Paths, root+v0alpha1.DashboardResourceInfo.GroupResource().Resource)
	delete(oas.Paths.Paths, root+"watch/"+v0alpha1.DashboardResourceInfo.GroupResource().Resource)

	// The root API discovery list
	sub := oas.Paths.Paths[root]
	if sub != nil && sub.Get != nil {
		sub.Get.Tags = []string{"API Discovery"} // sorts first in the list
	}
	return oas, nil
}

func (b *DashboardsAPIBuilder) GetAPIRoutes() *builder.APIRoutes {
	return nil // no custom API routes
}
