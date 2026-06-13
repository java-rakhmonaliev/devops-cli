# devopsify

One command to containerize, provision and ship any app — Django, FastAPI, Go,
Java or Node — to a lightweight VPS. Zero external dependencies (Go stdlib only),
compiles to a single static binary.

## Install

### Homebrew (recommended)

```sh
brew tap java-rakhmonaliev/tap https://github.com/java-rakhmonaliev/devops-cli
brew install java-rakhmonaliev/tap/devopsify
```

### From source

```sh
go build -o devopsify .
# put it on your PATH, e.g.
sudo mv devopsify /usr/local/bin/
```

## Use

From inside any project directory:

```sh
devopsify init --owner your-github-username   # detect + generate everything
devopsify detect                              # just print what it detected
devopsify plan                                # terraform plan
devopsify up                                  # terraform apply  (creates the VPS)
devopsify deploy                              # kamal deploy     (builds + ships)
devopsify destroy                             # terraform destroy
```

`init` is pure file generation — safe and idempotent (it skips files that
already exist; pass `--force` to overwrite). The stateful commands (`up`,
`deploy`, `destroy`) prompt for confirmation; pass `--yes` to skip.

## What `init` generates

```
Dockerfile                     # multi-stage, stack-specific (the only language-aware artifact)
.dockerignore
docker-compose.yml             # local dev; adds a Postgres service when needed
.env.example
.gitignore
terraform/                     # Hetzner Cloud VPS: server + firewall + docker via cloud-init
  main.tf  variables.tf  outputs.tf  cloud-init.yaml  terraform.tfvars.example
config/deploy.yml              # Kamal 2 deploy config
.github/workflows/deploy.yml   # build -> push to GHCR -> kamal deploy
.devopsify.json                # the profile: single source of truth, commit this
```

## The design (why this scales to any stack)

The container is the abstraction boundary. Once every app is "an image that
exposes a port and reads env vars," everything past the image is
language-agnostic. So the work is split into three layers:

1. **Profile** (`.devopsify.json`) — a small spec written by detection. Every
   generated file is a pure function of it, so the whole setup is regenerable
   and shows up as a reviewable diff.
2. **Generators** — profile -> files, no side effects, idempotent. Only the
   Dockerfile is stack-specific; compose, Terraform and CI are shared.
3. **Executors** — the stateful steps (`terraform apply`, `kamal deploy`),
   gated behind confirmation so provisioning never happens by accident.

## Extending it

- **New language**: add a Dockerfile template under `templates/docker/`, register
  it in `dockerTemplate` (generate.go), and add a detection branch (detect.go).
  Nothing else changes — the deploy pipeline doesn't care about the language.
- **New target** (AWS ECS, Cloud Run, K8s): add a Terraform module under
  `templates/`, switch on `profile.Target` in the generator. The container
  boundary means the image and CI stay the same.

## Manual steps (by design)

- App/deploy secrets go in GitHub repo settings (`SSH_PRIVATE_KEY`, `SECRET_KEY`, ...).
- `export TF_VAR_hcloud_token=...` before `up` (Hetzner Cloud API token).
- Fill in your domain and server IP in `config/deploy.yml`
  (`terraform -chdir=terraform output -raw server_ip`).

## Notes

- Default VPS is a Hetzner `cax11` (cheap ARM box). Swap the provider by
  replacing the `terraform/` templates — DigitalOcean/Linode are a near drop-in.
- For production, move Terraform state to a remote backend (commented stub in
  `main.tf`).
- This was scaffolded without a Go toolchain available to compile-test it, so
  run `go build` first and fix any typo before relying on it.
