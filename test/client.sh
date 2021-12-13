#!/bin/bash

set -exo pipefail

git clone https://github.com/pyke369/pdhcp.git
cd pdhcp
CGO_ENABLED=0 go build -o /usr/local/bin/pdhcp ./src/

apt update
apt -y install jq

# ./pdhcp -r localhost:6767 -i en7 -s 192.168.2.225
sleep infinity
