package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "detect":
		err = cmdDetect(args)
	case "plan":
		err = cmdPlan(args)
	case "up", "provision":
		err = cmdUp(args)
	case "deploy", "ship":
		err = cmdDeploy(args)
	case "destroy":
		err = cmdDestroy(args)
	case "version", "-v", "--version":
		fmt.Println("devopsify", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`devopsify — one command to containerize, provision and ship any app

USAGE
  devopsify <command> [flags]

SCAFFOLDING (safe, idempotent — pure file generation)
  init        Detect the stack and generate Dockerfile, compose, Terraform,
              Kamal config and a GitHub Actions workflow into the project.
  detect      Print the detected project profile as JSON and exit.

ACTIONS (stateful — prompt for confirmation unless --yes)
  plan        terraform plan for the provisioned VPS.
  up          terraform apply — create the server (alias: provision).
  deploy      kamal deploy — build, push and run the container (alias: ship).
  destroy     terraform destroy — tear the server down.

  version     Print version.

Run 'devopsify init -h' for init flags.
`)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	name := fs.String("name", "", "project name (default: detected / directory name)")
	owner := fs.String("owner", "CHANGEME", "registry owner, e.g. your GitHub username")
	port := fs.Int("port", 0, "container port (default: detected per stack)")
	stack := fs.String("stack", "", "override detected stack (django|fastapi|go|java|node)")
	db := fs.Bool("db", false, "force-add a Postgres service to docker-compose")
	force := fs.Bool("force", false, "overwrite files that already exist")
	fs.Parse(args)

	p := Detect(".")
	if *stack != "" {
		p.Stack = *stack
	}
	if *name != "" {
		p.Name = *name
	}
	if *port != 0 {
		p.Port = *port
	}
	if *db {
		p.NeedsDB = true
	}
	p.Registry = fmt.Sprintf("ghcr.io/%s/%s", *owner, p.Name)

	fmt.Printf("Detected: stack=%s framework=%s port=%d target=%s\n\n", p.Stack, dash(p.Framework), p.Port, p.Target)
	if err := Generate(p, *force); err != nil {
		return err
	}

	fmt.Print(`
Done. Next steps:
  1. Review the generated files, especially terraform/terraform.tfvars.example
     and config/deploy.yml (set your server IP / domain / GitHub username).
  2. export TF_VAR_hcloud_token=...   (Hetzner Cloud API token)
  3. devopsify up        # provisions the server
  4. devopsify deploy    # builds + ships the container

Secrets to add manually in the GitHub repo (Settings > Secrets):
  SSH_PRIVATE_KEY, plus any app secrets your deploy.yml references.
`)
	return nil
}

func cmdDetect(args []string) error {
	p := Detect(".")
	b, _ := json.MarshalIndent(p, "", "  ")
	fmt.Println(string(b))
	return nil
}

func cmdPlan(args []string) error {
	if _, err := LoadProfile(); err != nil {
		return err
	}
	if err := tfInit(); err != nil {
		return err
	}
	return run("terraform", "-chdir=terraform", "plan")
}

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	fs.Parse(args)
	if _, err := LoadProfile(); err != nil {
		return err
	}
	if !confirm("provision cloud resources with `terraform apply`", *yes) {
		return fmt.Errorf("aborted")
	}
	if err := tfInit(); err != nil {
		return err
	}
	return run("terraform", "-chdir=terraform", "apply")
}

func cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	fs.Parse(args)
	if _, err := LoadProfile(); err != nil {
		return err
	}
	if !confirm("build, push and deploy the container with `kamal deploy`", *yes) {
		return fmt.Errorf("aborted")
	}
	return run("kamal", "deploy")
}

func cmdDestroy(args []string) error {
	fs := flag.NewFlagSet("destroy", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	fs.Parse(args)
	if _, err := LoadProfile(); err != nil {
		return err
	}
	if !confirm("DESTROY all provisioned infrastructure with `terraform destroy`", *yes) {
		return fmt.Errorf("aborted")
	}
	return run("terraform", "-chdir=terraform", "destroy")
}

func tfInit() error {
	return run("terraform", "-chdir=terraform", "init", "-input=false")
}

func run(name string, args ...string) error {
	fmt.Println("+", name, strings.Join(args, " "))
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	return c.Run()
}

func confirm(action string, yes bool) bool {
	if yes {
		return true
	}
	fmt.Printf("About to %s.\nType 'yes' to continue: ", action)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(line)) == "yes"
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
