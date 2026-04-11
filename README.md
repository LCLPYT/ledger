# ledger
A service to manage unlockables, stats, digital currencies and other things for games.

This project features:
- a lightweight backend server written in Go
- an optional web dashboard written with Nuxt.js

The backend server provides REST endpoints that can be used to manage game and player data.

Most endpoints require authentication.
The backend implements Role-based-authentication for users.
API auth is handled via access tokens with granular permissions / scopes.
For the web dashboard, short-lived stateless session tokens are used.

## Setup
### Prerequisites
- [Docker](https://www.docker.com/) or [Podman](https://podman.io/) for running containers
- [Just](https://github.com/casey/just) to run saved commands easily

### Running locally
First, run first time setup:
```bash
# initialize default roles
just init_roles

# create test user (make it admin for testing when prompted)
just create_user
```

## Contributing - AI policy
Using generative AI to write code / assist with coding is fine in the scope of this project.
However, a human must always be involved in reviewing and submitting the changes.
I.e. before creating a PR, a human must first review and approve the changes before submitting the PR.