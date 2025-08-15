Veracode Go Workspace / multi-module packager Proof Of Concept
===

Proof of concept on how Veracode Auto-packaging might package multi-module / workspace projects for improved results.

## Download
On Linux with go get:

```
export GOPATH=`go env GOPATH` &&
export PATH="$GOPATH/bin:$PATH" &&
go install github.com/relaxnow/vc-gowork-poc/cmd/vc-gowork-poc@latest
```

## Run

```
vc-gowork-poc path/to/project
```

Will result in `project.zip` being produced.

## Run from local clone:
```
go run cmd/vc-gowork-poc/main.go path/to/project 
```

Will result in `project.zip` being produced.