# Palimpsest

PAL-uhmp-sest — /ˈpæl.ɪmp.sɛst/

A self-hosted note-taking app with hierarchical pages, rich-text editing, full-text search, and revision history with diffs. Go backend (chi + SQLite), Svelte/Vite frontend, embedded into a single binary via `go:embed`.

> **Why "Palimpsest"?** A palimpsest is a manuscript page that's been scraped clean and written over, with traces of the earlier text still visible underneath. That's a fitting description of what this app actually stores: every edit layers on top of the last, and nothing is truly erased - you can always dig back through a page's revision history to see what it used to say.

## Development

```bash
just dev    # run backend (:8080) and frontend dev server (:5173) together
just build  # build frontend + Go binary into ./palimpsest
just run    # build and run
just test   # run tests
```

See `justfile` for the full list of commands.
