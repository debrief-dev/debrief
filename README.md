# Debrief

Hyper-charged reverse search

## Development

Project follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/#specification) naming.

Prerequisites:

- [Go](https://go.dev/) 1.25.0
- [golangci-lint](https://golangci-lint.run/)


```sh
# Build & Run:
go run .

# Run tests
go test ./...

# Run linter
golangci-lint run --fix
```

## Architecture

Dependencies point downward: Application → Infrastructure → Domain. Exception is `data/shell` → `infra/platform` for OS detection.

```
Application Layer:
  main.go       -> lifecycle
  app/          -> application state
  ui/           -> immediate-mode GUI rendering

  Infrastructure Layer:
  infra/
    config/     -> configuration, persistence
    platform/   -> OS-specific path expansion, file ops, platform-detection
    hotkey/     -> global hotkey registration
    window/     -> window show/hide controller
    tray/       -> system tray icon and menu

  Domain Layer:
  data/
    model/      -> entities & value objects
    cmdstore/   -> aggregate: index, search, store
    syntax/     -> domain service: shell command parsing
    shell/      -> domain service: history file ingestion
    tree/       -> domain service: prefix tree build & flatten
    search/     -> domain service: fuzzy search, trigram index, scoring
```

## License

Copyright © 2026 bosiakov

Licensed under MIT (see [LICENSE](LICENSE)).

The font `font/FiraCode-Regular.ttf` is licensed under the OFL-1.1. See [font/LICENSE](font/LICENSE) for details.