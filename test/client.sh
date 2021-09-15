#!/bin/bash

set -exo pipefail

git clone https://github.com/pyke369/pdhcp.git
cd pdhcp
CGO_ENABLED=0 go build -o /usr/local/bin/pdhcp ./src/

apt update
apt -y install jq

sleep infinity
