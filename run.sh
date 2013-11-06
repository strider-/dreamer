#!/bin/bash

go build ./dreamer.go
nohup ./dreamer > ./dreamer.log 2>&1 &
echo "The dream is alive\n"