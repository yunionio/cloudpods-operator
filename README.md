# onecloud-operator

## Introduction

See: [./docs/intro.md](./docs/intro.md)

## Development

### Compile

```bash
$ git clone https://github.com/yunionio/onecloud-operator $GOPATH/src/yunion.io/x/onecloud-operator
$ cd $GOPATH/src/yunion.io/x/onecloud-operator
$ make
```

### Install code-generator

```bash
$ git clone https://github.com/kubernetes/code-generator $GOPATH/src/k8s.io/x/code-generator
$ cd $GOPATH/src/k8s.io/x/code-generator
$ git checkout kubernetes-1.15.1
$ go install go install ./cmd/{defaulter-gen,client-gen,lister-gen,informer-gen,deepcopy-gen}
```

Try ./hack/codegen.sh

```bash
$ ./hack/codegen.sh
```

## telegraf-init

```bash
VERSION=release-1.5 make telegraf-init-image
```
