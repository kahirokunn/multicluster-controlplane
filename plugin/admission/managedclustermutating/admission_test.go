// Copyright Contributors to the Open Cluster Management project
package managedclustermutating

import (
	"context"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	ocmfeature "open-cluster-management.io/api/feature"
	"open-cluster-management.io/ocm/pkg/features"
	runtimeadmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestDefaultClusterSetUsesInjectedAuthorizationClient(t *testing.T) {
	if err := features.HubMutableFeatureGate.Add(ocmfeature.DefaultHubRegistrationFeatureGates); err != nil {
		t.Fatal(err)
	}
	featuregatetesting.SetFeatureGateDuringTest(t, features.HubMutableFeatureGate, ocmfeature.DefaultClusterSet, true)
	for _, tc := range []struct {
		name       string
		allowed    bool
		clusterSet string
	}{{name: "authorized", allowed: true, clusterSet: "default"}, {name: "unauthorized"}} {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := kubefake.NewClientset()
			kubeClient.PrependReactor("create", "subjectaccessreviews", func(action clienttesting.Action) (bool, runtime.Object, error) {
				review := action.(clienttesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				if review.Spec.User != "bootstrap-user" || review.Spec.ResourceAttributes.Name != "default" {
					t.Fatalf("unexpected authorization request: %#v", review.Spec)
				}
				review.Status.Allowed = tc.allowed
				return true, review, nil
			})
			plugin := NewPlugin()
			plugin.SetExternalKubeClientSet(kubeClient)
			ctx := runtimeadmission.NewContextWithRequest(context.Background(), runtimeadmission.Request{AdmissionRequest: admissionv1.AdmissionRequest{UserInfo: authenticationv1.UserInfo{Username: "bootstrap-user"}}})
			cluster := &clusterv1.ManagedCluster{ObjectMeta: metav1.ObjectMeta{Name: "member"}}
			if err := plugin.webhook.Default(ctx, cluster); err != nil {
				t.Fatal(err)
			}
			if got := cluster.Labels[clusterv1beta2.ClusterSetLabel]; got != tc.clusterSet {
				t.Fatalf("cluster set label = %q, want %q", got, tc.clusterSet)
			}
		})
	}
}
