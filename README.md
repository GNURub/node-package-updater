# node-package-updater

**node-package-updater upgrades your package.json dependencies to the _latest_ versions, ignoring specified versions.**

## Installation

```bash
go install github.com/GNURub/node-package-updater/cmd/npu
```

## Usage

```bash
A CLI application to manage dependencies

Usage:
  npu [flags]

Flags:
  -c, --config string           Path to config file (default: .npmrc)
  -d, --directory string        Root directory for package search (default ".")
  -D, --dryRun                  Show what would be updated without making changes
  -e, --exclude strings         Packages to exclude (can be specified multiple times)
  -h, --help                    help for npu
  -I, --include strings         Packages to include (can be specified multiple times)
  -i, --includePeer             Include peer dependencies
  -k, --keepRange               Keep range operator on version (default true)
  -l, --log string              Log level (debug, info, warn, error) (default "info")
  -m, --minor                   Update to latest minor versions
  -n, --noInstall               Do not install packages after updating
  -x, --nonInteractive          Non-interactive mode
  -M, --packageManager string   Package manager to use (npm, yarn, pnpm, bun)
  -p, --patch                   Update to latest patch versions
  -P, --production              Update only production dependencies
  -r, --registry string         NPM registry URL (default "https://registry.npmjs.org/")
  -s, --semanticVersion         Maintain semver satisfaction
  -t, --timeout int             Timeout in seconds for each package update (default 30)
  -V, --verbose                 Show detailed output
  -v, --version                 Show version
  -w, --workspaces              Include workspace repositories
```
