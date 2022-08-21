#!/bin/bash

# get latest tag
tag=$(git rev-list --tags --max-count=1)

# if there are none, start tags at 0.0.0
if [ -z "$tag" ]; then
  TagSign=0.0.0
else
  TagSign=$(git describe --tags $tag)
fi

# get current commit hash for tag
commit=$(git rev-parse HEAD)

export TZ=Asia/Shanghai
export CGO_ENABLED=0
export GOOS=linux

LDFLAGS="\
-X \"github.com/Ivlyth/process-bandwidth/version.VERSION=${TagSign}\" \
-X \"github.com/Ivlyth/process-bandwidth/version.COMMIT=$(git rev-parse HEAD)\" \
-X \"github.com/Ivlyth/process-bandwidth/version.GOVERSION=$(go version)\" \
-X \"github.com/Ivlyth/process-bandwidth/version.COMPILE_AT=$(date +'%F %H:%M:%S')\" \
"

go build --ldflags "${LDFLAGS}" --gcflags "-N -l" -o bin/process-bandwidth .
