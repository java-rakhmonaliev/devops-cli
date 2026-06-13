package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates
var templatesFS embed.FS

// dockerTemplate maps a stack to its Dockerfile template. This map is the only
// place the deploy pipeline cares about language — everything past the image
// (compose, terraform, CI) is stack-agnostic.
var dockerTemplate = map[string]string{
	"django":  "templates/docker/django.Dockerfile.tmpl",
	"fastapi": "templates/docker/fastapi.Dockerfile.tmpl",
	"go":      "templates/docker/go.Dockerfile.tmpl",
	"java":    "templates/docker/java.Dockerfile.tmpl",
	"node":    "templates/docker/node.Dockerfile.tmpl",
}

// render uses [[ ]] delimiters so that {{ }} and ${{ }} in the generated
// Dockerfiles, compose files, HCL and GitHub Actions pass through untouched.
func render(tmplPath string, p *Profile) ([]byte, error) {
	src, err := templatesFS.ReadFile(tmplPath)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(tmplPath)).Delims("[[", "]]").Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", tmplPath, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, p); err != nil {
		return nil, fmt.Errorf("execute %s: %w", tmplPath, err)
	}
	return buf.Bytes(), nil
}

func writeFile(path string, data []byte, force bool) error {
	if !force && exists(path) {
		fmt.Printf("  skip   %s (exists)\n", path)
		return nil
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("  write  %s\n", path)
	return nil
}

// Generate writes all DevOps artifacts for the profile. It is idempotent:
// existing files are left alone unless force is set.
func Generate(p *Profile, force bool) error {
	dt, ok := dockerTemplate[p.Stack]
	if !ok {
		return fmt.Errorf("no Dockerfile template for stack %q (supported: django, fastapi, go, java, node)", p.Stack)
	}

	jobs := []struct{ tmpl, out string }{
		{dt, "Dockerfile"},
		{"templates/dockerignore.tmpl", ".dockerignore"},
		{"templates/gitignore.tmpl", ".gitignore"},
		{"templates/env.example.tmpl", ".env.example"},
		{"templates/compose.yml.tmpl", "docker-compose.yml"},
		{"templates/terraform/main.tf.tmpl", "terraform/main.tf"},
		{"templates/terraform/variables.tf.tmpl", "terraform/variables.tf"},
		{"templates/terraform/outputs.tf.tmpl", "terraform/outputs.tf"},
		{"templates/terraform/cloud-init.yaml.tmpl", "terraform/cloud-init.yaml"},
		{"templates/terraform/tfvars.example.tmpl", "terraform/terraform.tfvars.example"},
		{"templates/kamal.deploy.yml.tmpl", "config/deploy.yml"},
		{"templates/github.deploy.yml.tmpl", ".github/workflows/deploy.yml"},
	}

	for _, j := range jobs {
		data, err := render(j.tmpl, p)
		if err != nil {
			return err
		}
		if err := writeFile(j.out, data, force); err != nil {
			return err
		}
	}

	if err := p.Save(); err != nil {
		return err
	}
	fmt.Printf("  write  %s\n", profilePath)
	return nil
}
