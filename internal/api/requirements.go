package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RequirementItem is one role/collection a project pulls in, classified by where
// it comes from so the UI can deep-link to it (a galaxy/git page, or the local
// file editor).
type RequirementItem struct {
	Kind    string `json:"kind"` // collection | role
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source"`         // galaxy | git | local
	URL     string `json:"url,omitempty"`  // external link (galaxy/git)
	Path    string `json:"path,omitempty"` // project-relative file to open in the editor
}

type requirementsResponse struct {
	Files []string          `json:"files"`
	Items []RequirementItem `json:"items"`
}

// galaxyEntry parses a requirements.yml list item, which may be a bare scalar
// ("- community.general") or a mapping ("- name: x\n  src: …").
type galaxyEntry struct {
	Name    string `yaml:"name"`
	Src     string `yaml:"src"`
	Version string `yaml:"version"`
}

func (e *galaxyEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		e.Name = value.Value
		return nil
	}
	type raw galaxyEntry
	var r raw
	if err := value.Decode(&r); err != nil {
		return err
	}
	*e = galaxyEntry(r)
	return nil
}

var requirementsFiles = []string{
	"requirements.yml", "requirements.yaml",
	"roles/requirements.yml", "collections/requirements.yml",
}

// handleProjectRequirements lists the galaxy roles/collections a project installs
// (from its requirements files) plus any local role folders, each classified by
// source so the UI can link to it.
func (s *Server) handleProjectRequirements(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	dir := s.projectDir(p)
	resp := requirementsResponse{Files: []string{}, Items: []RequirementItem{}}
	seen := map[string]bool{}
	add := func(it RequirementItem) {
		key := it.Kind + "|" + it.Name
		if it.Name == "" || seen[key] {
			return
		}
		seen[key] = true
		resp.Items = append(resp.Items, it)
	}

	for _, rf := range requirementsFiles {
		data, err := os.ReadFile(filepath.Join(dir, rf))
		if err != nil {
			continue
		}
		resp.Files = append(resp.Files, rf)
		var doc struct {
			Collections []galaxyEntry `yaml:"collections"`
			Roles       []galaxyEntry `yaml:"roles"`
		}
		if err := yaml.Unmarshal(data, &doc); err == nil && (len(doc.Collections) > 0 || len(doc.Roles) > 0) {
			for _, c := range doc.Collections {
				add(classifyRequirement("collection", c, dir))
			}
			for _, ro := range doc.Roles {
				add(classifyRequirement("role", ro, dir))
			}
			continue
		}
		// A top-level sequence (no collections:/roles: keys) is a roles list.
		var list []galaxyEntry
		if err := yaml.Unmarshal(data, &list); err == nil {
			for _, ro := range list {
				add(classifyRequirement("role", ro, dir))
			}
		}
	}

	// Local roles: folders under roles/ even if not named in a requirements file.
	if entries, err := os.ReadDir(filepath.Join(dir, "roles")); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				add(RequirementItem{Kind: "role", Name: e.Name(), Source: "local", Path: localRolePath(dir, e.Name())})
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func classifyRequirement(kind string, e galaxyEntry, dir string) RequirementItem {
	it := RequirementItem{Kind: kind, Name: e.Name, Version: e.Version}
	src := strings.TrimSpace(e.Src)
	if src != "" {
		url := strings.TrimPrefix(src, "git+")
		switch {
		case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"):
			it.Source = "git"
			it.URL = strings.TrimSuffix(url, ".git")
		case strings.HasPrefix(src, "git@"):
			it.Source = "git"
			it.URL = sshToHTTPS(src)
		default:
			it.Source = "local"
			it.Path = src
			if it.Name == "" {
				it.Name = lastSegment(src)
			}
			return it
		}
		if it.Name == "" {
			it.Name = lastSegment(it.URL)
		}
		return it
	}
	// No src: a local role folder, or a galaxy "namespace.name".
	if kind == "role" {
		if fi, err := os.Stat(filepath.Join(dir, "roles", e.Name)); err == nil && fi.IsDir() {
			it.Source = "local"
			it.Path = localRolePath(dir, e.Name)
			return it
		}
	}
	if ns := strings.SplitN(e.Name, ".", 2); len(ns) == 2 {
		it.Source = "galaxy"
		if kind == "collection" {
			it.URL = "https://galaxy.ansible.com/ui/repo/published/" + ns[0] + "/" + ns[1] + "/"
		} else {
			it.URL = "https://galaxy.ansible.com/ui/standalone/roles/" + ns[0] + "/" + ns[1] + "/"
		}
		return it
	}
	it.Source = "galaxy"
	it.URL = "https://galaxy.ansible.com/ui/search/?keywords=" + e.Name
	return it
}

// localRolePath returns the best file to open for a local role (its tasks/main,
// else the role directory).
func localRolePath(dir, name string) string {
	for _, cand := range []string{
		"roles/" + name + "/tasks/main.yml",
		"roles/" + name + "/tasks/main.yaml",
		"roles/" + name + "/main.yml",
	} {
		if _, err := os.Stat(filepath.Join(dir, cand)); err == nil {
			return cand
		}
	}
	return "roles/" + name
}

func lastSegment(s string) string {
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// sshToHTTPS turns git@github.com:org/repo.git into https://github.com/org/repo.
func sshToHTTPS(s string) string {
	s = strings.TrimSuffix(strings.TrimPrefix(s, "git@"), ".git")
	s = strings.Replace(s, ":", "/", 1)
	return "https://" + s
}
