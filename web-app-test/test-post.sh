#!/usr/bin/env bash
curl -i -X PUT http://localhost:8001/api/v1/namespaces/kubemod-system/services/kubemod-webapp-service:api/proxy/v1/dryrun -H "Content-Type: application/json" -d @test-post.json
