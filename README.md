# predictable-yaml

## Status
This is alpha, but ready for use.

## Usage
* `go run main.go --help` OR
* `go build main.go -o predictable-yaml`
* `predictable-yaml --help` OR
* `go install`
* `predictable-yaml --help`
* Setup a config dir
    * `mkdir ~/.predictable-yaml && cp ./example-configs/* ~/.predictable-yaml/`

Linting Examples:
* `predictable-yaml lint --config-dir example-configs $(find test-data -name '*\.yaml')`
* `predictable-yaml lint --config-dir example-configs --quiet $(find test-data -name '*\.valid.yaml')`
* `docker run -it --rm -v $(pwd):/code -w /code snarlysodboxer/predictable-yaml:latest lint --config-dir example-configs $(find test-data -name '*.yaml')`

Fixer Examples:
_See caviats below about fixing_
* `predictable-yaml fix --config-dir example-configs $(find test-data -name '*\.yaml')`
* `predictable-yaml fix --config-dir example-configs --indentation-level 2 --reduce-list-indentation-by 2 $(find test-data -name '*\.yaml')`
* `predictable-yaml fix --config-dir example-configs --prompt-if-line-count-change $(find test-data -name '*\.yaml')`
* `docker run -it --rm -v $(pwd):/code -w /code snarlysodboxer/predictable-yaml:latest fix --config-dir example-configs $(find test-data -name '*.yaml')`

## Functionality
* Uses one config schema file per target file schema. E.G. One config file for a Kubernetes `Deployment` is used to lint any Deployment target files.
    * Can be used with any YAML schema, not just Kubernetes ones.
* Indentation does not have to match between the config file and target files. Use another linter for that.
* Algorithm
    * Lines in the target file that exist in the config file must be in the same order as the config file, but there can be any amount of lines inbetween them.
    * Lines can be configured to be first, required, or to 'ditto' the configs under another line.

## Config files
* Supports multiple config schema files. Place them in the config directory.
    * Config type can be configured with the comment `# predictable-yaml: kind=my-schema`, but if this is not found, we'll attempt to get it from the Kubernetes-esq `kind: my-schema`, value. If neither are found, an error will be thrown.
    * Target file type will be determined in the same way, however if a matching config is not found, a warning will be output.
* Config files must:
    * Not have comments other than the configuration ones specific to this program.
    * Not have more than one entry in each sequence.
        * The first entry in a sequence in the config will be used for each entry in the target file.
    * Not have null nodes, and empty node types must match what's expected in the target file.
        * Good: `initContainers: []  # ditto=.spec.containers`.
        * Not good: `initContainers:  # ditto=.spec.containers`.
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
* A file can be skipped with the comment `# predictable-yaml: ignore`.
* A required lines can be ignored with the comment `# predictable-yaml: ignore-requireds`.
* These can be combined: `# predictable-yaml: kind=my-schema, ignore-requireds`.

## Fixing
* Support for fixing is limited by Golang yaml.v3's ability to record and reproduce comments.
    * __*It is possible to loose comments altogether with the fixer, if the comment is not a header, footer, or line comment.*__ (Empty lines around the comment define these.)
    * Recommend starting with a clean Git tree so changes can easily be undone if the results are undesired. (Or use the diff to fix any dropped comments, which will hopefully be rare-ish.)
* Prompting and not prompting for confirmation are both supported.
* Lines in the target file that are not in the config file will be moved to the end by default.
    * Change to the begining with the flag `--unmatched-to-beginning`.
* Missing required keys will be added unless `ignore-requireds` is set as a per file override.
* Lines can be set to `# preferred` to allow linting to pass without them, but make the fixer add them when fixing, unless the `--ignore-preferred` flag is set.

## Tests
* Run with `go test ./...`.

## Debugging with Delve
* `dlv test pkg/compare/compare* -- -test.run TestWalkAndCompare`
