# devopsify — project context

A single-binary CLI that containerizes, provisions, and ships any app
(Django, FastAPI, Go, Java, Node) to a lightweight VPS. Solo/personal tool.

## Core principle

**The container is the abstraction boundary.** Once every app is "an OCI image
that exposes a port and reads env vars," everything past the image is
language-agnostic. This keeps the work linear (N Dockerfile templates + M deploy
targets) instead of an N×M matrix. Preserve this boundary in all changes.

## Architecture — three layers

1. **Profile** (`.devopsify.json`) — single source of truth, written by detection,
   committed to the repo. Every generated file is a *pure function* of it.
2. **Generators** — `profile -> files`. No side effects, idempotent (skip existing
   files unless `--force`). Only the Dockerfile is stack-specific; compose,
   Terraform, and CI are shared/language-blind.
3. **Executors** — stateful steps (`terraform apply`, `kamal deploy`), each gated
   behind a `yes` confirmation so provisioning never happens by accident.

## Technical choices (keep these)

- **Go, stdlib only.** No external deps — compiles to a static binary, no
  `go mod download`. Don't add deps without strong reason.
- **Profile = JSON** via `encoding/json` (not YAML, to stay dependency-free).
- **Templates use `[[ ]]` delimiters** (`template.Delims("[[","]]")`) so `{{ }}`
  and `${{ }}` in Docker/compose/HCL/GitHub Actions pass through untouched.
  Templates are `//go:embed`-ed from `templates/`.
- **Target: Hetzner Cloud VPS** (`cax11` ARM box), Docker installed via
  cloud-init; firewall opens 22/80/443.
- **Deploy: Kamal 2**; **registry: GHCR**; **CI: GitHub Actions** (build → push → deploy).

## Layout

```
main.go        # subcommand dispatch (stdlib flag), init + executors
detect.go      # stack detection -> Profile
profile.go     # Profile struct, JSON load/save
generate.go    # embedded templates, render (custom delims), idempotent writes
templates/     # docker/<stack>.Dockerfile.tmpl, compose, terraform/, kamal, github actions, env, ignores
```

## Commands

- `init` — detect + generate everything (safe, idempotent). Flags: `--owner`,
  `--name`, `--port`, `--stack`, `--db`, `--force`.
- `detect` — print detected profile JSON (read-only).
- `plan` / `up`(=provision) / `deploy`(=ship) / `destroy` — executors, `--yes` to skip prompt.

## Extension points

- **New language**: add `templates/docker/<x>.Dockerfile.tmpl`, register in
  `dockerTemplate` map (generate.go), add a detection branch (detect.go).
  Nothing downstream changes.
- **New target** (AWS ECS, Cloud Run, K8s): add a Terraform module under
  `templates/`, switch on `profile.Target` in the generator.

## Manual by design

Secret *values* (`SSH_PRIVATE_KEY`, `SECRET_KEY`, …) go in GitHub repo settings;
`export TF_VAR_hcloud_token=...` before `up`; fill domain + server IP in
`config/deploy.yml`. The tool wires everything else.

## Open TODOs (discussed, not yet done)

1. **Compile-verify** — scaffolded without a Go toolchain; run `go build` and fix
   any typos before relying on it.
2. **Remote Terraform state** — currently local; commented backend stub in
   `terraform/main.tf`. Add S3/DynamoDB or Hetzner object-storage backend +
   handle the state-bootstrap chicken-and-egg.
3. **Auto-wire server IP** — `deploy` should read `terraform output -raw server_ip`
   and inject it into Kamal instead of the `<SERVER_IP>` placeholder.
4. **More targets** — e.g. Cloud Run, plus DigitalOcean/Linode as near-drop-in
   VPS module swaps.
