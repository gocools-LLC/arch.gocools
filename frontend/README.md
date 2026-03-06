# Arch Frontend

Interactive AWS architecture editor for `arch.gocools`.

## Features (v0)

- drag and drop AWS resources onto a canvas
- pan/zoom board interaction
- select and move nodes
- connect nodes with dependency edges
- live load from `GET /api/v1/graph`
- plan preview from `POST /api/v1/graph/diff`
- guarded stack operations from `POST /api/v1/stacks/operations`
- guardrail panel for required tags and destroy confirmations
- tag stamping utility for required GoCools tags
- JSON export/import for diagrams
- node inspector for metadata and required GoCools tags

## Run

```bash
cd frontend
npm install
npm run dev
```

Dev server runs on `http://localhost:3005` and proxies `/api` to `http://127.0.0.1:8081`.

## Build

```bash
npm run build
npm run preview
```

## Notes

- This editor UI is intentionally API-first for `arch.gocools`.
- Interaction model is inspired by `../frontend/edit.gocools` and can later be branded as powered by it.
