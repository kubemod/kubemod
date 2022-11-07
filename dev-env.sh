#!/usr/bin/env bash

# Copy the kubemod webhook certificate files from the kubemod pod down to our dev machine.
echo "Copying webhook TLS certs from secret to local machine..."
crt_dir=/tmp/k8s-webhook-server/serving-certs
mkdir -p $crt_dir
kubectl get secret webhook-server-cert -o=jsonpath="{.data['tls\.crt']}" -n kubemod-system | base64 -d > $crt_dir/tls.crt
kubectl get secret webhook-server-cert -o=jsonpath="{.data['tls\.key']}" -n kubemod-system | base64 -d > $crt_dir/tls.key

# Start telepresence.
echo "Starting telepresence..."
telepresence intercept kubemod-operator --namespace kubemod-system --port 9443:443
