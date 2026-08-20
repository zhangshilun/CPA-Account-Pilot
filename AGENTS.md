# CPA Account Pilot

Standalone CLIProxyAPI native plugin for persistent private-account management and fast login actions.

## Commands

```bash
gofmt -w *.go internal/**/*.go web/*.go
go test ./...
make build
make verify
```

## Conventions

- Keep the CGO ABI bridge thin; business logic belongs under `internal/`.
- Use exact CPA Management API routes and keep resource routes static-only.
- Never log or return Management Keys, auth JSON, tokens, cookies, API keys, or proxy credentials.
- Private-account passwords are encrypted by the plugin service. The web page may keep a local copy only to support the explicit copy-password action.
- Public response models must be allow-listed and contain no raw credential material.
