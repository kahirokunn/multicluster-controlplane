# Copyright Contributors to the Open Cluster Management project
#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/init.sh"

for f in $HUB_CRD_FILES
do
    cp $f ./pkg/controllers/bootstrap/crds/
done

for f in $SPOKE_CRD_FILES
do
    cp $f ./pkg/agent/crds/
done

# Go vendoring excludes the Cluster Inventory API CRD manifests.
go mod download sigs.k8s.io/cluster-inventory-api
cluster_inventory_api_dir=$(go list -m -mod=mod -f '{{.Dir}}' sigs.k8s.io/cluster-inventory-api)
cp "${cluster_inventory_api_dir}/config/crd/bases/multicluster.x-k8s.io_clusterprofiles.yaml" \
    ./pkg/controllers/bootstrap/crds/0000_00_multicluster.x-k8s.io_clusterprofiles.crd.yaml
