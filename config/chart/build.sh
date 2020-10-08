#!/bin/sh
set -e


# Parse arguments and set variables
if [ -z "$1" ] || [ -z "$2" ]; then
    echo "usage: $(basename "$0") [CHART_FORMAT] [CHART_VERSION]"
    exit 1
fi
CHART_FORMAT=$1
CHART_VERSION=$2
WORKSPACE=${WORKSPACE:-/workspace}


# Post process the deployment manifest
templatizeDeployment() {
    sed '/namespace: redsky-system/d' | \
        sed 's/SECRET_SHA256/{{ include (print $.Template.BasePath "\/secret.yaml") . | sha256sum }}/g' | \
        sed 's/RELEASE_NAME/"{{ .Release.Name }}"/g' | \
        sed 's/VERSION/"{{ .Chart.AppVersion }}"/g' | \
        sed 's/IMG:TAG/"{{ .Values.redskyImage }}:{{ .Values.redskyTag }}"/g' | \
        sed 's/PULL_POLICY/{{ .Values.redskyImagePullPolicy }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g'
}

# Post process the RBAC manifest
templatizeRBAC() {
    sed 's/namespace: redsky-system/namespace: {{ .Release.Namespace | quote }}/g' | \
        sed 's/name: redsky-\(.*\)$/name: "{{ .Release.Name }}-\1"/g' | \
        sed '1i\{{- if .Values.rbac.create -}}' | \
        sed '$a{{- end -}}'
}

# Post processing to add recommended labels
# https://github.com/koalaman/shellcheck/issues/1246
# shellcheck disable=SC1004
label() {
    sed '/creationTimestamp: null/d' | \
    sed '/^  labels:$/,/^    app\.kubernetes\.io\/name: redskyops$/c\
  labels:\
    app.kubernetes.io/name: redskyops\
    app.kubernetes.io/version: "{{ .Chart.AppVersion }}"\
    app.kubernetes.io/instance: "{{ .Release.Name }}"\
    app.kubernetes.io/managed-by: "{{ .Release.Service }}"\
    helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}"'
}


# For the default root we remove the resources that get included separately.
cd "$WORKSPACE/default"
kustomize edit remove resource "../crd"
kustomize edit remove resource "../rbac"


# For the manager we need to replace the image name with something that will
# match the filters later.
cd "$WORKSPACE/manager"
kustomize edit set image controller="IMG:TAG"


# For the CRD resources we need to add back the "name" label so the label filters
# will find it. We do not add the Helm CRD hook annotation because the CRD isn't
# used during the installation process.
cd "$WORKSPACE/crd"
kustomize edit add label "app.kubernetes.io/name:redskyops"


# For the RBAC resources we need to add back the "name" label so the label filters
# will find it and because we removed it from the default root, we need to add
# back the name prefix and namespace transformations.
cd "$WORKSPACE/rbac"
kustomize edit add label "app.kubernetes.io/name:redskyops"
kustomize edit set namespace "redsky-system"
kustomize edit set nameprefix "redsky-"

# TODO Have an option to include the ServiceMonitor?

# Build the templates for the chart
cd "$WORKSPACE"
kustomize build crd | label > "$WORKSPACE/chart/redskyops/templates/crds.yaml"
kustomize build rbac | templatizeRBAC | label > "$WORKSPACE/chart/redskyops/templates/rbac.yaml"
kustomize build chart | templatizeDeployment | label > "$WORKSPACE/chart/redskyops/templates/deployment.yaml"


# Remove icon reference from Rancher chart, Rancher specific files from everything else
if [ "$CHART_FORMAT" = "rancher" ] ; then
    sed -i 's|^icon: .*/icon.png$|icon: file://../icon.png|' "$WORKSPACE/chart/redskyops/Chart.yaml"
else
    rm "$WORKSPACE/chart/redskyops/app-readme.md" "$WORKSPACE/chart/redskyops/questions.yml"
fi


# Package everything together using Helm
helm package --version "${CHART_VERSION}" "$WORKSPACE/chart/redskyops" > /dev/null


# Output the chart in the expected format
case "$CHART_FORMAT" in
helm)
    tar c -z -C "$WORKSPACE" "redskyops-$CHART_VERSION.tgz" | base64
    ;;
rancher)
    BASE="$WORKSPACE/chart/rancher"
    DEST="$BASE/charts/redskyops/v$CHART_VERSION"
    mkdir -p "$DEST"
    tar x -z --strip-components 1 -f "$WORKSPACE/redskyops-$CHART_VERSION.tgz" -C "$DEST"
    tar c -z -C "$BASE" . | base64
    ;;
digitalocean)
    BASE="$WORKSPACE/chart/digitalocean"
    DEST="$BASE/src/redskyops/$CHART_VERSION"
    mkdir -p "$DEST"
    tar x -z --strip-components 1 -f "$WORKSPACE/redskyops-$CHART_VERSION.tgz" -C "$DEST"
    tar c -z -C "$BASE" . | base64
    ;;
*)
    echo "Unknown chart format: $CHART_FORMAT"
    exit 1
esac
