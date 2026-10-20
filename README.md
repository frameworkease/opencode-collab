# OpenCode Collab

[![DeepWiki](https://img.shields.io/badge/docs-DeepWiki-blue)](https://deepwiki.com/frameworkease/opencode-collab)
[![Release](https://img.shields.io/github/v/tag/frameworkease/opencode-collab?logo=github)](https://github.com/frameworkease/opencode-collab/tags)

Sidecar in front of OpenCode: reverse-proxy, collab identity, message `metadata.sender`, timeline avatars.

## One pair link

1. Mint an OpenCode pair: `opencode pair --url <publicBase>`
2. Bind it to a collab user: `POST /opencode-collab/v1/bind` with that `code` + `user` (optional `publicBase` → response `connectUrl`)
3. Client opens `{publicBase}/auth/connect/{code}` — OpenCode login; if bound, also sets `opencode_collab_sid` for `sender` / avatar

`OPENCODE_COLLAB_UPSTREAM` → OpenCode main. `OPENCODE_COLLAB_UPSTREAM_PAIR` → OpenCode pair service (required for connect).

Local smoke: see [`tmp/README.md`](tmp/README.md) (`./tmp/smoke.sh up` / `link`).
