# forma.flow

This repository is split by responsibility.

- `frontend` contains the FORMA TELA static frontend and frontend documentation.
- `main` is reserved for backend development.

The frontend branch is the source of truth for browser UI and the DemoAPI contract.

## Backend architecture branch

Go/PostgreSQL architecture is maintained separately in [`backend-architecture`](https://github.com/yomezza/forma.flow/tree/backend-architecture/docs/backend), with a [documentation index](docs/backend/README.md) and [cross-branch links to the actual frontend sources](docs/backend/frontend-source-links.md).

The existing JavaScript implementation stays in [`frontend`](https://github.com/yomezza/forma.flow/tree/frontend); [DemoAPI](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L1-L345) is the current demo, **not** the Go server. `main` and `frontend` must not be modified by architecture-only work.
