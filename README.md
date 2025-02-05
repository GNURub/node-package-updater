# node-package-updater

**node-package-updater upgrades your package.json dependencies to the _latest_ versions, ignoring specified versions.**

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/GNURub/node-package-updater/main/install.sh | bash
```

```bash
go install github.com/GNURub/node-package-updater/cmd/npu@latest
```

or

```bash
npm install -g node-package-updater
```

## Usage

```bash
A CLI application to manage NodeJs dependencies

Usage:
  npu [flags]
  npu [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  global      Global dependencies update
  help        Help about any command
  upgrade     Upgrade to the latest version of the CLI
  version     Print the version number of NPU

Flags:
  -C, --cleanCache              Clean cache
  -c, --config string           Path to config file (default: .npmrc)
  -d, --dir string              Root directory for package search
  -D, --dryRun                  Show what would be updated without making changes
  -e, --exclude strings         Packages to exclude (can be specified multiple times)
  -f, --filter string           Filter packages by name regex
  -h, --help                    help for npu
  -I, --include strings         Packages to include (can be specified multiple times)
  -i, --includePeer             Include peer dependencies
  -k, --keepRange               Keep range operator on version (default true)
      --log string              Log level (debug, info, warn, error) (default "info")
  -m, --minor                   Update to latest minor versions
  -n, --noInstall               Do not install packages after updating
  -x, --nonInteractive          Non-interactive mode
  -M, --packageManager string   Package manager to use (npm, yarn, pnpm, bun)
  -p, --patch                   Update to latest patch versions
      --pre                     Update to latest versions
  -P, --production              Update only production dependencies
  -r, --registry string         NPM registry URL (default "https://registry.npmjs.org/")
  -s, --semanticVersion         Maintain semver satisfaction
      --skipDeprecated          Skip deprecated packages (default true)
  -t, --timeout int             Timeout in seconds for each package update (default 30)
  -V, --verbose                 Show detailed output
  -w, --workspaces              Include workspace repositories
```

## Benchmarks

| Command     |      Mean [ms] | Min [ms] | Max [ms] |    Relative |
| :---------- | -------------: | -------: | -------: | ----------: |
| `npu -x -n` |  851.7 ± 194.2 |    699.7 |   1320.8 |        1.00 |
| `ncu -u`    | 2572.8 ± 292.2 |   2227.6 |   3065.2 | 3.02 ± 0.77 |

### Summary

npu -x -n ran
3.02 ± 0.77 times faster than ncu -u
