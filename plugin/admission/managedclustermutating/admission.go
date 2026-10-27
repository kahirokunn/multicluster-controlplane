// Copyright Contributors to the Open Cluster Management project
package managedclustermutating

import (
	"context"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apiserver/pkg/admission"
	genericadmissioninitializer "k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/generic"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/request"
	"k8s.io/client-go/kubernetes"
	clusterv1api "open-cluster-management.io/api/cluster/v1"
	clusterwebhookv1 "open-cluster-management.io/ocm/pkg/registration/webhook/v1"
	runtimeadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionutil "open-cluster-management.io/multicluster-controlplane/plugin/admission/util"
)

const PluginName = "ManagedClusterMutating"

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return NewPlugin(), nil
	})
}

type Plugin struct {
	*admission.Handler
	webhook *clusterwebhookv1.ManagedClusterWebhook
}

func (p *Plugin) SetExternalKubeClientSet(client kubernetes.Interface) {
	p.webhook.SetExternalKubeClientSet(client)
}

func (p *Plugin) ValidateInitialization() error {
	return nil
}

var _ admission.MutationInterface = &Plugin{}
var _ admission.InitializationValidator = &Plugin{}
var _ genericadmissioninitializer.WantsExternalKubeClientSet = &Plugin{}

func NewPlugin() *Plugin {
	return &Plugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		webhook: &clusterwebhookv1.ManagedClusterWebhook{},
	}
}

func (p *Plugin) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	v := admission.VersionedAttributes{
		Attributes:         a,
		VersionedOldObject: a.GetOldObject(),
		VersionedObject:    a.GetObject(),
		VersionedKind:      a.GetKind(),
	}

	gvr := clusterv1api.Resource("managedclusters").WithVersion("v1")
	gvk := schema.GroupVersionKind{
		Group:   gvr.Group,
		Version: gvr.Version,
		Kind:    "ManagedCluster",
	}

	// resource is not mcl
	if a.GetKind() != gvk {
		return nil
	}

	// we just need gvr in code logic
	i := generic.WebhookInvocation{
		Kind:     gvk,
		Resource: gvr,
	}

	uid := types.UID(uuid.NewUUID())
	ar := request.CreateV1AdmissionReview(uid, &v, &i)

	old := a.GetOldObject()
	oldRaw := runtime.RawExtension{}
	err := admissionutil.Convert_runtime_Object_To_runtime_RawExtension_Raw(&old, &oldRaw)
	if err != nil {
		return fmt.Errorf("error occured in ManagedClusterMutating: failed to convert Object to RawExtension")
	}
	ar.Request.OldObject = oldRaw

	cluster := &clusterv1api.ManagedCluster{}
	obj := a.GetObject().(*unstructured.Unstructured)
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster)
	if err != nil {
		return err
	}

	r := runtimeadmission.Request{AdmissionRequest: *ar.Request}
	admissionContext := runtimeadmission.NewContextWithRequest(ctx, r)
	if err := p.webhook.Default(admissionContext, cluster); err != nil {
		return err
	}

	updated, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster)
	if err != nil {
		return err
	}
	obj.Object = updated

	return nil
}
