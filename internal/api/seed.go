package api

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nikiv/ansible-ui/examples"
	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

// builtinApps are the execution backends seeded on every boot (configurable
// afterwards via the Applications admin page).
var builtinApps = []model.Application{
	{ID: "ansible", Name: "Ansible", Icon: "ansible", Priority: 1, Active: true, Kind: model.AppKindBuiltin},
	{ID: "terraform", Name: "Terraform", Icon: "terraform", Priority: 2, Active: true, Kind: model.AppKindBuiltin},
	{ID: "tofu", Name: "OpenTofu", Icon: "tofu", Priority: 3, Active: true, Kind: model.AppKindBuiltin},
	{ID: "terragrunt", Name: "Terragrunt", Icon: "terragrunt", Priority: 4, Active: true, Kind: model.AppKindBuiltin},
	{ID: "bash", Name: "Bash", Icon: "bash", Priority: 5, Active: true, Kind: model.AppKindBuiltin},
	{ID: "powershell", Name: "PowerShell", Icon: "powershell", Priority: 6, Active: true, Kind: model.AppKindBuiltin},
	{ID: "python", Name: "Python", Icon: "python", Priority: 7, Active: true, Kind: model.AppKindBuiltin},
}

// EnsureApplications seeds the built-in apps if missing (idempotent; preserves
// admin edits). Runs on every boot regardless of SeedDemo.
func (s *Server) EnsureApplications(ctx context.Context) {
	for i := range builtinApps {
		a := builtinApps[i]
		if err := s.store.CreateApplicationIfMissing(ctx, &a); err != nil {
			s.log.Warn("ensure application failed", "id", a.ID, "err", err)
		}
	}
}

// Seed materialises the embedded demo project into the data volume and creates
// its project, default inventory and starter templates — once, on first boot.
func (s *Server) Seed(ctx context.Context) error {
	s.EnsureApplications(ctx)
	if !s.cfg.SeedDemo {
		return nil
	}
	if existing, err := s.store.GetProjectBySlug(ctx, "demo"); err == nil {
		// Already seeded — idempotently upgrade older installs with the newer
		// multi-app demo files and starter templates.
		s.ensureDemoMultiApp(ctx, existing)
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	dest := filepath.Join(s.cfg.DataDir, "projects", "demo")
	if err := extractEmbedded(examples.Project, examples.Root, dest); err != nil {
		return err
	}

	p := &model.Project{
		Slug:        "demo",
		Name:        "Demo · Ansible Examples",
		Description: "A localhost-safe tour of Ansible: from a one-line hello to a multi-role, tag-driven deployment. Run any playbook to see the live terminal.",
		Path:        "projects/demo",
		SourceType:  "local",
	}
	if err := s.store.CreateProject(ctx, p); err != nil {
		return err
	}

	// A file inventory pointing at the project's inventory.ini (rather than an
	// inline static copy) so runs use `-i inventory.ini` from the project dir —
	// which keeps the command clean AND lets ansible discover the project's
	// group_vars/host_vars (they live beside it).
	inv := &model.Inventory{
		ProjectID: p.ID,
		Name:      "localhost (demo)",
		Type:      model.InventoryFile,
		Content:   "inventory.ini",
	}
	if err := s.store.CreateInventory(ctx, inv); err != nil {
		return err
	}

	starters := []struct {
		name, playbook, desc string
		verbosity            int // -vv on the richer demos so the log shows task paths + the config banner (parity with Semaphore's verbose templates)
	}{
		{"Hello World", "playbooks/01-hello.yml", "Your first green run — ping + banner.", 0},
		{"Gather Facts", "playbooks/02-facts.yml", "Discover the managed node and set a custom fact.", 0},
		{"Variables & Loops", "playbooks/03-variables-loops.yml", "Loops, conditionals, register and assert.", 0},
		{"Files & Templates", "playbooks/04-files-and-templates.yml", "copy / template / lineinfile with handlers.", 0},
		{"Roles", "playbooks/06-roles.yml", "Compose common, webserver and monitoring roles.", 2},
		{"Error Handling", "playbooks/07-blocks-rescue.yml", "block / rescue / always recovery.", 0},
		{"Full-Stack Deploy", "playbooks/10-full-stack.yml", "End-to-end, tag-driven deployment with a verification gate.", 2},
	}
	for _, t := range starters {
		tpl := &model.Template{
			ProjectID:   p.ID,
			Name:        t.name,
			Description: t.desc,
			Playbook:    t.playbook,
			InventoryID: &inv.ID,
			ExtraVars:   map[string]any{},
			Diff:        true,
			Verbosity:   t.verbosity,
		}
		if err := s.store.CreateTemplate(ctx, tpl); err != nil {
			return err
		}
	}

	// Multi-app starters — show off the non-Ansible execution backends.
	for _, a := range demoMultiAppStarters {
		tpl := &model.Template{
			ProjectID:   p.ID,
			Name:        a.name,
			Description: a.desc,
			App:         a.app,
			Action:      a.action,
			Playbook:    a.entry,
			ExtraVars:   map[string]any{},
		}
		if err := s.store.CreateTemplate(ctx, tpl); err != nil {
			return err
		}
	}

	s.log.Info("seeded demo project", "project", p.ID, "templates", len(starters)+len(demoMultiAppStarters))
	return nil
}

// demoMultiAppStarters are the non-Ansible demo templates.
var demoMultiAppStarters = []struct {
	name, app, action, entry, desc string
}{
	{"Terraform · plan", model.AppTerraform, "plan", "terraform", "terraform_data demo — init + plan (localhost-safe, no cloud creds)."},
	{"Terraform · apply", model.AppTerraform, "apply", "terraform", "Apply the terraform_data demo with a local-exec echo."},
	{"Pulumi · preview", model.AppPulumi, "preview", "pulumi", "Pulumi YAML demo — preview against a local backend (no cloud login)."},
	{"Bash · hello", model.AppBash, "", "scripts/hello.sh", "Run a Bash script in the live PTY (colour + TTY preserved)."},
	{"Python · report", model.AppPython, "", "scripts/report.py", "Run a Python script and print host facts."},
}

// ensureDemoMultiApp brings a previously-seeded demo project up to date: it
// materialises the multi-app example files (if missing) and creates any
// multi-app starter templates that don't exist yet. Non-destructive.
func (s *Server) ensureDemoMultiApp(ctx context.Context, p *model.Project) {
	dir := s.projectDir(p)
	_, tfErr := os.Stat(filepath.Join(dir, "terraform", "main.tf"))
	_, puErr := os.Stat(filepath.Join(dir, "pulumi", "Pulumi.yaml"))
	if tfErr != nil || puErr != nil {
		// Old seed missing some multi-app files — re-materialise the embedded tree
		// (restores the canonical demo content; extractEmbedded only adds missing files).
		if err := extractEmbedded(examples.Project, examples.Root, dir); err != nil {
			s.log.Warn("ensure demo multi-app: extract failed", "err", err)
		}
	}
	existing, err := s.store.ListTemplates(ctx, p.ID)
	if err != nil {
		return
	}
	have := map[string]bool{}
	for _, t := range existing {
		have[t.Name] = true
	}
	added := 0
	for _, a := range demoMultiAppStarters {
		if have[a.name] {
			continue
		}
		tpl := &model.Template{
			ProjectID:   p.ID,
			Name:        a.name,
			Description: a.desc,
			App:         a.app,
			Action:      a.action,
			Playbook:    a.entry,
			ExtraVars:   map[string]any{},
		}
		if err := s.store.CreateTemplate(ctx, tpl); err == nil {
			added++
		}
	}
	if added > 0 {
		s.log.Info("upgraded demo with multi-app starters", "added", added)
	}
}

// extractEmbedded copies an embedded FS subtree to a destination directory.
func extractEmbedded(efs fs.FS, root, dest string) error {
	return fs.WalkDir(efs, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// Non-destructive: never overwrite a file that already exists (so a
		// re-materialise to add new demo files can't clobber the user's edits).
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		data, err := fs.ReadFile(efs, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
