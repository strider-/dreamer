#!/bin/bash

go build ./dreamer.go -fcgi
nohup ./dreamer > ./dreamer.log 2>&1 &
echo "The dream is alive."