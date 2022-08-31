# predictable-yaml

## Status
This is an early work in progress, not ready for use yet.

## Usage
* `go run main.go --help`

Examples:
* `find test-data -name '*\.yaml' | go run main.go --config-dir example-configs`
* `find test-data -name '*\.valid.yaml' | go run main.go --config-dir example-configs --quiet`
* `find test-data -name '*.yaml' | docker run -i --rm -v $(pwd):/code -w /code snarlysodboxer/predictable-yaml:latest --config-dir example-configs`

## Algorithm
* Indentation must match, between the config file and target files.
* For each line in the config file, find any matching target file lines and validate, recursively.
* Lines in the target file must be somewhere after the line number matched by the config's line number minus one's config, not nessarily immediately after.* (This ensures a consistent order while allowing comments and unaccounted for objects keys.)
* Lines set to `first` in the config must be first in the target file's Map.
* Lines set to `required` in the config must exist in the target file. This matches on the key's name and column number.
* Lines set to `ditto` must pass the same validation as the configs under another key.
* Lines set to `none` are cataloged as needed but have no other requirements. This is useful when a key is not required, but if it's included, certain sub-keys should be required.

## Config files
* Supports multiple config schema files. Place them in the config directory.
    * Config type can be configured with the comment `# predictable-yaml-kind: my-schema`.
        * If this is not found, we'll attempt to get it from the Kubernetes-esq `kind: my-schema`, value.
    * Target file type will be determined in the same way.
* Must not have comments other than the configuration ones specific to this program.
* Set `first` to throw errors if a key is not first.
* Set `required` to throw errors if a key is not found.
* Set `ditto` to setup a key and sub-keys with the same configs as another key and sub-keys.

## Test
* Run with `go test ./...`.
