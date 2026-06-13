package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readLower(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToLower(string(b))
}

// Detect inspects the working directory and returns a best-effort Profile.
// Detection is intentionally cheap and overridable: flags on `init` win, and
// the user can always hand-edit .devopsify.json afterwards.
func Detect(dir string) *Profile {
	p := &Profile{Name: filepath.Base(mustAbs(dir)), Target: "hetzner-vps"}

	switch {
	case exists(filepath.Join(dir, "go.mod")):
		p.Stack = "go"
		p.Port = 8080
		p.BinaryName = goBinaryName(dir)

	case exists(filepath.Join(dir, "pom.xml")) || exists(filepath.Join(dir, "build.gradle")) || exists(filepath.Join(dir, "build.gradle.kts")):
		p.Stack = "java"
		p.Port = 8080
		if exists(filepath.Join(dir, "pom.xml")) {
			p.JavaBuildTool = "maven"
		} else {
			p.JavaBuildTool = "gradle"
		}

	case exists(filepath.Join(dir, "manage.py")):
		p.Stack = "django"
		p.Port = 8000
		p.NeedsDB = true
		p.WSGIModule = djangoWSGI(dir, p.Name)

	case isPython(dir) && mentionsFastAPI(dir):
		p.Stack = "fastapi"
		p.Port = 8000
		p.AppModule = fastAPIAppModule(dir)

	case isPython(dir):
		// Plain Python service; treat like fastapi-style runner by default.
		p.Stack = "fastapi"
		p.Port = 8000
		p.AppModule = fastAPIAppModule(dir)

	case exists(filepath.Join(dir, "package.json")):
		p.Stack = "node"
		p.Port = 3000
		p.Framework = nodeFramework(dir)

	default:
		p.Stack = "unknown"
		p.Port = 8080
	}

	return p
}

func mustAbs(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

func isPython(dir string) bool {
	return exists(filepath.Join(dir, "requirements.txt")) ||
		exists(filepath.Join(dir, "pyproject.toml")) ||
		exists(filepath.Join(dir, "Pipfile"))
}

func mentionsFastAPI(dir string) bool {
	for _, f := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		c := readLower(filepath.Join(dir, f))
		if strings.Contains(c, "fastapi") || strings.Contains(c, "uvicorn") {
			return true
		}
	}
	return false
}

func fastAPIAppModule(dir string) string {
	candidates := []struct{ file, mod string }{
		{"main.py", "main:app"},
		{"app.py", "app:app"},
		{"app/main.py", "app.main:app"},
		{"src/main.py", "src.main:app"},
	}
	for _, c := range candidates {
		if exists(filepath.Join(dir, c.file)) {
			return c.mod
		}
	}
	return "main:app"
}

// djangoWSGI finds the package that holds wsgi.py (one level deep) so the
// gunicorn entrypoint is correct without manual editing.
func djangoWSGI(dir, fallback string) string {
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && exists(filepath.Join(dir, e.Name(), "wsgi.py")) {
				return e.Name() + ".wsgi"
			}
		}
	}
	return fallback + ".wsgi"
}

func goBinaryName(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimSpace(strings.TrimPrefix(line, "module "))
				parts := strings.Split(mod, "/")
				name := parts[len(parts)-1]
				if name != "" {
					return name
				}
			}
		}
	}
	return filepath.Base(mustAbs(dir))
}

func nodeFramework(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return ""
	}
	has := func(k string) bool {
		_, a := pkg.Dependencies[k]
		_, d := pkg.DevDependencies[k]
		return a || d
	}
	switch {
	case has("next"):
		return "next"
	case has("nuxt"):
		return "nuxt"
	case has("express"):
		return "express"
	case has("fastify"):
		return "fastify"
	default:
		return ""
	}
}
