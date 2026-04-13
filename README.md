# ledger
A service to manage unlockable items, game statistics, digital currencies and other things. 

This project features:
- a lightweight backend server written in Go
- an optional web dashboard written with Nuxt.js

The backend server provides REST endpoints that can be used to manage game and player data.

Most endpoints require authentication.
The backend implements Role-based-authentication for users.
API auth is handled via access tokens with granular permissions / scopes.
For the web dashboard, short-lived stateless session tokens are used.

> [!NOTE]
> In this project I am trying out the capabilities of Claude Code and the viability to use in projects like this.
> This involves testing limits and finding boundaries on its capabilities on my use case.
> Thus, the codebase may not be as "clean" or refactored as I would like it to be.

## Backend service
### Prerequisites
- [Go](https://go.dev/)
- [Docker](https://www.docker.com/) or [Podman](https://podman.io/) for running containers
- [Just](https://github.com/casey/just) to run saved commands easily

### One time setup
First, run first time setup:
```bash
# initialize default roles
just init_roles

# create test user (make it admin for testing when prompted)
just create_user
```

### Running
```bash
just serve
```

### Executing tests
```bash
just test
```

## Web dashboard
### Prerequisites
- [Node.js](https://nodejs.org)
- [NPM](https://www.npmjs.com/)
- [Just](https://github.com/casey/just) to run saved commands easily

### Running
Make sure to run the backend in another terminal, or the dashboard won't be all that useful. 😀
```bash
just front
```

## Contributing - AI policy
Using generative AI to write code / assist with coding is fine in the scope of this project.
However, a human must always be involved in reviewing and submitting the changes.
I.e. before creating a PR, a human must first review and approve the changes before submitting the PR.
