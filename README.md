# predictable-yaml

`predictable-yaml` lints and fixes the order of keys in YAML files based on configurable schemas. It ships with built-in configs for many Kubernetes resource types and supports custom schemas for any YAML format.

## Quick Start

```shell
# Install via Nix flake
nix profile install github:snarlysodboxer/predictable-yaml

# Or download from GitHub releases
# https://github.com/snarlysodboxer/predictable-yaml/releases/latest

# Lint a directory (uses built-in Kubernetes configs by default)
predictable-yaml lint my-k8s-manifests/

# Fix key ordering in-place (shows diff and prompts before writing)
predictable-yaml fix my-k8s-manifests/
```

Out of the box, `predictable-yaml` knows how to lint and fix Deployments, Services, Pods, ConfigMaps, CronJobs, Ingresses, and [many more Kubernetes types](https://github.com/snarlysodboxer/predictable-yaml-configs). No configuration required.

## Install

### Nix Flake
```shell
# Run without installing
nix run github:snarlysodboxer/predictable-yaml -- lint --help

# Install to profile
nix profile install github:snarlysodboxer/predictable-yaml

# Development shell (includes Go, gopls, delve)
nix develop github:snarlysodboxer/predictable-yaml

# Build locally from a checkout
nix build
./result/bin/predictable-yaml --help
```

### Binary Release
Download the [latest release](https://github.com/snarlysodboxer/predictable-yaml/releases/latest) for your platform.
```shell
mv ~/Downloads/predictable-yaml-linux-amd64 /usr/local/bin/predictable-yaml
chmod +x /usr/local/bin/predictable-yaml
# On macOS, remove the quarantine attribute
sudo xattr -r -d com.apple.quarantine /usr/local/bin/predictable-yaml
```

### Docker
```shell
docker run -it --rm -v $(pwd):/code -w /code \
  snarlysodboxer/predictable-yaml:latest lint my-dir/
```

### From Source
```shell
go generate ./internal/embedded/  # fetch default configs
go build -o predictable-yaml
# or
go install
```

## Config Sources

`predictable-yaml` resolves configs in this order of precedence:

1. **`--config-dir` flag** - If specified, only configs from that directory are used.
2. **Local `.predictable-yaml/` directories** - Searched up the directory tree from the working directory and down into target paths. Subdirectory configs override parent configs for files under that path.
3. **Remote configs** - Fetched from a git repository specified in `.predictable-yaml/.remote`.
4. **Built-in defaults** - Kubernetes/Kustomize schemas embedded in the binary.

For most Kubernetes projects, the built-in defaults work without any configuration. For custom schemas or to pin a specific config version, use remote configs.

### Remote Configs

Instead of maintaining `.predictable-yaml` config directories in every repo, point to a remote config repository. Create a `.predictable-yaml/.remote` file:

```yaml
# Uses the default config repo (github.com/snarlysodboxer/predictable-yaml-configs)
version: v1.0.0
```

```yaml
# Custom repo, pinned to a tag
remote: https://github.com/my-org/my-yaml-configs
version: v2.3.0
```

The `version` field accepts any git ref: a tag, commit SHA, or branch name.

Configs are fetched once and cached locally in `.predictable-yaml/.cache/`. When the version is bumped in `.remote`, the cache is automatically updated on the next run.

- `.predictable-yaml/.cache/` should be gitignored.
- `.predictable-yaml/.remote` should be committed.
- Local config files in `.predictable-yaml/` still override remote configs.
- Nested `.remote` files in subdirectories work the same as nested local configs.
- GitHub, GitLab, and Bitbucket are supported. Private repos use `GITHUB_TOKEN`, `GITLAB_TOKEN`, or `BITBUCKET_TOKEN` env vars, falling back to local git credentials.

### Show Active Configs

To see which configs will be used for a given path:

```shell
predictable-yaml show-configs my-dir/
```

## Linting

```shell
# Lint a directory tree
predictable-yaml lint my-dir/

# Lint specific files
predictable-yaml lint deployment.yaml service.yaml

# Suppress success messages
predictable-yaml lint --quiet my-dir/

# Use a specific config directory
predictable-yaml lint --config-dir ./my-configs my-dir/
```

Pass directory paths to search recursively for YAML files, file paths to check specific files, or any combination.

## Fixing

The fixer reorders keys to match the config schema. By default, it shows a diff and prompts for confirmation before writing changes.

```shell
# Fix with diff + prompt (default)
predictable-yaml fix my-dir/

# Fix without prompting
predictable-yaml fix --prompt=false my-dir/

# Only prompt if the line count changes
predictable-yaml fix --prompt-if-line-count-change my-dir/

# Use four spaces and more deeply indented lists
predictable-yaml fix --indentation-level 4 --reduce-list-indentation-by 0 my-dir/

# Disable whitespace preservation and list de-indentation
predictable-yaml fix -d my-dir/
```

### Fixer Features

- **Key reordering** - Reorders keys to match the config schema. Always produces valid YAML.
- **Preserve empty lines** - Associates empty lines with the YAML key following them and reinserts them after reordering. Reinserts trailing empty lines if present in the original. *(enabled by default, disable with `--preserve-empty-lines=false`)*
- **Preserve comments** - Replaces comment spacing with the original versions after reordering. *(enabled by default, disable with `--preserve-comments=false`)*
- **Reduce list indentation** - Adjusts list item indentation so `-` is even with the parent key instead of indented. *(enabled by default, disable with `--reduce-list-indentation-by 0`)*
- **Add missing keys** - Adds required keys that are missing from the file. Preferred keys can also be added with `--add-preferred`.
- **Unmatched key placement** - Keys in the file that aren't in the config are moved to the end of their map by default. Use `--unmatched-to-beginning` to move them to the start instead.
- **Document marker** - Reinserts `---` at the beginning of the file if it was there before reordering.

### Notes

- Recommend starting with a clean git tree so changes can easily be reviewed or undone.
- Whitespace preservation is not perfect in every circumstance, as it involves inferring intent from empty lines. Disable with `--preserve-empty-lines=false` if results are unexpected.
- Comment preservation may not work well when the fixer also changes indentation from the original. Set `--indentation-level` and `--reduce-list-indentation-by` to match your project's style first.
- Comment support is limited by Go's yaml.v3 library. It is possible for comments to be lost if they are not header, footer, or line comments (empty lines around a comment define these).

## Writing Config Files

Config files define the expected key order for a YAML schema. See [example-configs](./example-configs) for examples, or browse the [default configs](https://github.com/snarlysodboxer/predictable-yaml-configs).

Config files can be used with any YAML schema, not just Kubernetes.

### Schema Detection

Config file schema is set with `# predictable-yaml: kind=my-schema`. If not found, the `kind:` field value is used (Kubernetes convention). Target files are matched the same way.

### Config Directives

Add these as comments on config keys:

| Directive | Effect |
|-----------|--------|
| `# first` | Key must be first in its map |
| `# required` | Key must exist (fixer adds it if missing) |
| `# preferred` | Fixer adds it when `--add-preferred` is set |
| `# ditto=.path.to.node` | Reuse config from another node |

Combine directives: `# first, required, ditto=Pod.spec`

#### Ditto References

- **Local path** (starts with `.`): `# ditto=.spec.template.spec.containers`
- **Cross-schema** (starts with kind): `# ditto=Pod.spec`

### Config File Rules

- No comments other than the directive comments listed above.
- No more than one entry in each sequence (the first entry is used as the template for all entries in target files).
- No null nodes; node types must match what's expected in target files.

Good:
```yaml
initContainers: []  # ditto=.spec.containers
containers: []  # required
```

Not good:
```yaml
initContainers:  # ditto=.spec.containers
containers: []  # required
```

### Per-File Overrides

Target files can include these comments before, inline with, or after the first line of YAML:

- `# predictable-yaml: kind=my-schema` - Override schema detection
- `# predictable-yaml: ignore` - Skip this file
- `# predictable-yaml: ignore-requireds` - Skip required key checks
- Combine: `# predictable-yaml: kind=my-schema, ignore-requireds`

## Building

```shell
# Build for current platform
go generate ./internal/embedded/  # fetch default configs
go build -o predictable-yaml

# Run tests
go test ./...
```

## Debugging with Delve

```shell
dlv test pkg/compare/compare* -- -test.run TestWalkAndCompare
```
