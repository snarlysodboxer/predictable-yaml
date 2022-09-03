# predictable-yaml

## Status
This is alpha, but ready for use.

## Usage
* `go run main.go --help` OR
* `go build main.go -o predictable-yaml`
* `predictable-yaml --help` OR
* `go install`
* `predictable-yaml --help`

Examples:
* `find test-data -name '*\.yaml' | predictable-yaml --config-dir example-configs`
* `find test-data -name '*\.valid.yaml' | predictable-yaml --config-dir example-configs --quiet`
* `find test-data -name '*.yaml' | docker run -i --rm -v $(pwd):/code -w /code snarlysodboxer/predictable-yaml:latest --config-dir example-configs`

## Functionaltiy
* Supports multiple config schema files.
* Indentation does not have to match between the config file and target files. Use another linter for that.
* Lines in the target file that exist in the config file must be in the same order as the config file, but there can be any amount of lines inbetween them.

## Config files
* Supports multiple config schema files. Place them in the config directory.
    * Config type can be configured with the comment `# predictable-yaml: kind=my-schema`, but if this is not found, we'll attempt to get it from the Kubernetes-esq `kind: my-schema`, value. If neither are found, an error will be thrown.
    * Target file type will be determined in the same way, however if a matching config is not found, a warning will be output.
* Config files must not have comments other than the configuration ones specific to this program.
* Set `# first` to throw errors if a key is not first.
* Set `# required` to throw errors if a key is not found.
* Set `# ditto=[kind].<path>` to setup a key and sub-keys with the same configs as another key and sub-keys.
    * Point to a key in the same config file by starting with a `.`, like: `ditto=.spec.template.spec.containers`.
    * Point to a key in a different config file by starting with a schema type, then the path to the node, like: `ditto=Pod.spec`.
* Set any combination of these like: `# first, required, ditto=Pod.spec`.
* See [example-configs](./example-configs) for examples.

## Configuring per file overrides
__Config comments in target files must be before, inline, or after the first line of yaml (not just at the top of the file.)__
* File type can be configured or overridden with the comment `# predictable-yaml: kind=my-schema`.
* A single file can be skipped with the comment `# predictable-yaml: ignore-file`.
* A single file can be skipped with the comment `# predictable-yaml: ignore-requireds`.
* These can be combined: `# predictable-yaml: kind=my-schema, ignore-requireds`.

## Tests
* Run with `go test ./...`.
