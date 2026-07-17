# Inkbase

A self-hosted note-taking app with hierarchical pages, rich-text editing, full-text search, and revision history with diffs. Go backend (chi + SQLite), Svelte/Vite frontend, embedded into a single binary via `go:embed`.

## Development

```bash
just dev    # run backend (:8080) and frontend dev server (:5173) together
just build  # build frontend + Go binary into ./inkbase
just run    # build and run
just test   # run tests
```

See `justfile` for the full list of commands.
