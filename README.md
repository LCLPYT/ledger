# ledger
A service to manage unlockables, stats and digital currencies for games.

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
