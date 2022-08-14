# predictable-yaml

## Status
This is an early work in progress, not ready for use yet.

## Usage
Run like:
* `echo test-data/deployment.valid.yaml | go run main.go`
* `echo test-data/deployment.invalid-labels.yaml | go run main.go`
* Or:
```shell
echo 'test-data/deployment.valid.yaml
test-data/deployment.invalid-labels.yaml' | go run main.go
```
