#!/usr/bin/env bash
curl -i -X POST http://localhost:8081/v1/test -H "Content-Type: application/json" -d @test-post.json
