package api

import (
	"reflect"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// modelDoc documents an API data model (entity) for the Explorer's Models tab —
// the field list is reflected from the Go struct's json tags, so it never drifts.
type modelDoc struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Fields      []modelField `json:"fields"`
}

type modelField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

var apiModelDefs = []struct {
	Name, Desc string
	Sample     any
}{
	{"Project", "A tenant — a workspace scoping templates, inventories, runs and members.", model.Project{}},
	{"Template", "A saved run configuration (Task / Build / Deploy), launched ad-hoc or on a schedule.", model.Template{}},
	{"Inventory", "Hosts for a run: static text, a file/dynamic script, an HTTP URL, or a cloud source.", model.Inventory{}},
	{"Run", "One execution of a template or ad-hoc command, with status, output and stats.", model.Run{}},
	{"Workflow", "A sequential pipeline of templates with conditional (on_success/on_failure/always) steps.", model.Workflow{}},
	{"WorkflowRun", "One execution of a workflow, with per-step status + linked run ids.", model.WorkflowRun{}},
	{"Schedule", "A cron or one-time trigger that launches a template.", model.Schedule{}},
	{"Credential", "A Key Store entry: SSH key, login/password, vault password, or cloud service account.", model.Credential{}},
	{"NotificationChannel", "An outbound alert target (telegram/slack/teams/discord/…), optionally project-scoped.", model.NotificationChannel{}},
	{"Runner", "An execution agent (built-in or remote) with tags and a concurrency cap.", model.Runner{}},
	{"Environment", "Named environment variables + external secret references injected into a run.", model.Environment{}},
	{"Repository", "A git source feeding one or more projects.", model.Repository{}},
	{"User", "A local or SSO account with a global role.", model.User{}},
	{"Activity", "An audit-log entry — who did what, when.", model.Activity{}},
}

func (s *Server) apiModels() []modelDoc {
	out := make([]modelDoc, 0, len(apiModelDefs))
	for _, d := range apiModelDefs {
		out = append(out, modelDoc{Name: d.Name, Description: d.Desc, Fields: reflectModelFields(d.Sample)})
	}
	return out
}

func reflectModelFields(v any) []modelField {
	t := reflect.TypeOf(v)
	var fields []modelField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields = append(fields, modelField{Name: name, Type: goTypeName(f.Type)})
	}
	return fields
}

var timeType = reflect.TypeOf(time.Time{})

// goTypeName renders a Go field type as a friendly schema type (string, integer,
// boolean, timestamp, X[], X?, object, or a nested struct name).
func goTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return goTypeName(t.Elem()) + "?"
	case reflect.Slice, reflect.Array:
		return goTypeName(t.Elem()) + "[]"
	case reflect.Map:
		return "object"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.String:
		return "string"
	case reflect.Struct:
		if t == timeType {
			return "timestamp"
		}
		return t.Name()
	case reflect.Interface:
		return "any"
	default:
		return t.Kind().String()
	}
}
