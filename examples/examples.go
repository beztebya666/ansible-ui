// Package examples ships a complete, localhost-safe Ansible project that is
// embedded into the api binary and seeded into the data volume on first boot.
// It doubles as living documentation: a progression of playbooks from a trivial
// "hello" up to a multi-role, tag-driven end-to-end deployment.
package examples

import "embed"

// Project is the embedded Ansible demo project tree (everything under project/).
//
//go:embed all:project
var Project embed.FS

// Root is the path prefix inside the embedded FS.
const Root = "project"
