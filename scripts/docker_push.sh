#!/bin/bash

set -o errexit
set -o pipefail

if [ "$DEBUG" == "true" ] ; then
    set -ex ;export PS4='+(${BASH_SOURCE}:${LINENO}): ${FUNCNAME[0]:+${FUNCNAME[0]}(): }'
fi

readlink_mac() {
  cd `dirname $1`
  TARGET_FILE=`basename $1`

  # Iterate down a (possible) chain of symlinks
  while [ -L "$TARGET_FILE" ]
  do
    TARGET_FILE=`readlink $TARGET_FILE`
    cd `dirname $TARGET_FILE`
    TARGET_FILE=`basename $TARGET_FILE`
  done

  # Compute the canonicalized name by finding the physical path
  # for the directory we're in and appending the target file.
  PHYS_DIR=`pwd -P`
  REAL_PATH=$PHYS_DIR/$TARGET_FILE
}

pushd $(cd "$(dirname "$0")"; pwd) > /dev/null
readlink_mac $(basename "$0")
cd "$(dirname "$REAL_PATH")"
CUR_DIR=$(pwd)
SRC_DIR=$(cd .. && pwd)
popd > /dev/null

DOCKER_DIR="${DOCKER_DIR}"

REGISTRY=${REGISTRY:-docker.io/yunion}
TAG=${TAG:-latest}
PROJ=onecloud-operator
image_keyword=${IMAGE_KEYWORD}

build_bin() {
    local component="$1"; shift
    local BUILD_ARCH="$1";
    local BUILD_CC="$2";
    local BUILD_CGO="$3"

	docker run --rm \
        -v $SRC_DIR:/root/go/src/yunion.io/x/$PROJ \
        -v $SRC_DIR/_output/alpine-build:/root/go/src/yunion.io/x/$PROJ/_output \
        -v $SRC_DIR/_output/alpine-build/_cache:/root/.cache \
        registry.cn-beijing.aliyuncs.com/yunionio/alpine-build:3.22.0-go-1.24.6-1 \
        /bin/sh -c "set -ex; git config --global --add safe.directory /root/go/src/yunion.io/x/$PROJ; cd /root/go/src/yunion.io/x/$PROJ;
        $BUILD_ARCH $BUILD_CC $BUILD_CGO SHELL='sh -x' GOOS=linux make $component;
        chown -R $(id -u):$(id -g) _output;
        find _output/bin -type f |xargs ls -lah"
}

build_image() {
    local tag=$1
    local file=$2
    local path=$3
    docker build -t "$tag" -f "$2" "$3"
}

buildx_and_push() {
    local tag=$1
    local file=$2
    local path=$3
    local arch=$4
    docker buildx build -t "$tag" --platform "linux/$arch" -f "$2" "$3" --push
    docker pull --platform "linux/$arch" "$tag"
}

push_image() {
    local tag=$1
    docker push "$tag"
}

get_image_name() {
    local component=$1
    local arch=$2
    local is_all_arch=$3
    local img_name="$REGISTRY/$component:$TAG"
    if [[ "$is_all_arch" == "true" || "$arch" == arm64 ]]; then
        img_name="${img_name}-$arch"
    fi
    echo $img_name
}

build_process() {
    local component=$1
    local arch=$2
    local is_all_arch=$3
    local img_name=$(get_image_name $component $arch $is_all_arch)

    build_bin $component
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[$(readlink -f ${BASH_SOURCE}):${LINENO} ${FUNCNAME[0]}] return for DRY_RUN"
        return
    fi
    build_image $img_name $DOCKER_DIR/Dockerfile $SRC_DIR
    if [[ "$PUSH" == "true" ]]; then
        push_image "$img_name"
    fi
}

build_process_with_buildx() {
    local component=$1
    local arch=$2
    local is_all_arch=$3
    local img_name=$(get_image_name $component $arch $is_all_arch)

    build_env="GOARCH=$arch "
    if [[ $arch == arm64 ]]; then
        build_env="$build_env CC=aarch64-linux-musl-gcc"
    fi
	build_bin $component $build_env
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[$(readlink -f ${BASH_SOURCE}):${LINENO} ${FUNCNAME[0]}] return for DRY_RUN"
        return
    fi
	buildx_and_push $img_name $DOCKER_DIR/Dockerfile $SRC_DIR $arch
}

make_manifest_image() {
    local component=$1
    local img_name=$(get_image_name $component "" "false")
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[$(readlink -f ${BASH_SOURCE}):${LINENO} ${FUNCNAME[0]}] return for DRY_RUN"
        return
    fi
    docker buildx imagetools create -t $img_name \
        $img_name-amd64 \
        $img_name-arm64
}

cd $SRC_DIR

echo "Start to build for arch[$ARCH]"

ALL_COMPONENTS=$(ls cmd | grep -v '.*cli$' | xargs)

if [ "$#" -lt 1 ]; then
    echo "No component is specified~"
    echo "You can specify a component in [$ALL_COMPONENTS]"
    echo "If you want to build all components, specify the component to: all."
    exit
elif [ "$#" -eq 1 ] && [ "$1" == "all" ]; then
    echo "Build all onecloud docker images"
    COMPONENTS=$ALL_COMPONENTS
else
    COMPONENTS=$@
fi

echo COMPONENTS ${COMPONENTS[@]}
for component in $COMPONENTS; do
    case "$ARCH" in
        all)
            for arch in "arm64" "amd64"; do
                build_process_with_buildx $component $arch "true"
            done
            make_manifest_image $component
            ;;
        arm64)
            build_process_with_buildx $component $ARCH "false"
            ;;
        *)
            build_process $component $ARCH "false"
            ;;
    esac
done

show_update_cmd() {
    local component=$1
    local arch=$2
    local is_all_arch="false"
    if [[ $arch == all ]]; then
        is_all_arch="true"
    fi

    local img_name=$(get_image_name $component $arch $is_all_arch)
    echo "kubectl patch deployments -n onecloud onecloud-operator --type='json' -p='[{op: replace, path: /spec/template/spec/containers/0/image, value: ${img_name}}]'"
}


for component in $COMPONENTS; do
    show_update_cmd $component $ARCH
done
