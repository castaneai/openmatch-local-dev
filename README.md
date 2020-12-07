# Open Match Local Development example

A Local development example for [Open Match](https://open-match.dev).

![Overview](./overview.drawio.svg)

## Requirements

- [minikube](https://github.com/kubernetes/minikube)
- [skaffold](https://github.com/GoogleContainerTools/skaffold)
- Go
- GNU Make

## Install

```
make up-minikube
make up-openmatch
```

## Usage

```sh
make dev # make Match Function up
go run director/main.go # make Director up
go run testclient/main.go # run matchmaking
```
