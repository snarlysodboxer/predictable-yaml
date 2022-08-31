# predictable-yaml

## Status
This is an early work in progress, not ready for use yet.

## Usage
* `go run main.go --help`

Examples:
* `echo test-data/deployment.valid.yaml | go run main.go`
* `echo test-data/deployment.invalid-labels.yaml | go run main.go`
* Or:
```shell
echo 'test-data/deployment.valid.yaml
test-data/deployment.invalid-labels.yaml' | go run main.go
```
## Algorithm
* Indentation must match, between the config file and target files.
* For each line in the config file, find any matching target file lines and validate, recursively.
* Lines in the target file must be somewhere after the line number matched by the config's line number minus one's config, not nessarily immediately after.* (This ensures a consistent order while allowing comments and unaccounted for objects keys.)
* Lines set to `first` in the config must be first in the target file's Map.
* Lines set to `required` in the config must exist in the target file. This matches on the key's name and column number.
* Lines set to `ditto` must pass the same validation as the configs under another key.
* Lines set to `none` are cataloged as needed but have no other requirements. This is useful when a key is not required, but if it's included, certain sub-keys should be required.

## Config files
* Must not have comments other than the configuration ones specific to this program.
* Set `first` to throw errors if a key is not first.
* Set `required` to throw errors if a key is not found.
* Set `ditto` to setup a key and sub-keys with the same configs as another key and sub-keys.
