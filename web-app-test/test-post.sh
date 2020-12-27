#!/usr/bin/env bash
curl -i -X PUT http://localhost:8081/v1/dry-run -H "Content-Type: application/json" -d @test-post.json
