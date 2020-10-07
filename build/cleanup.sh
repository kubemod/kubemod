#!/usr/bin/env bash
trap "exit 0" ERR

docker image prune -f
