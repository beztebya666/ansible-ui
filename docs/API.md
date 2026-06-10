# API reference

Base URL: the api service (`http://localhost:8090` when exposed, or behind nginx at
`/api`). All bodies are JSON.

## REST

### Health, stats & metrics
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Liveness probe |
| GET | `/api/stats` | Dashboard: project/template/run counts, runs-by-state, recent runs |
| GET | `/metrics` | Prometheus exposition: runs by status, active/queued/awaiting, projects/templates, runners + per-runner load. Lives **outside `/api`** (no session guard) — public unless `METRICS_TOKEN` is set (then `Authorization: Bearer <token>`). |

### Projects
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects` | List projects |
| POST | `/api/projects` | Create `{name, description?, slug?}` |
| GET | `/api/projects/{id}` | Get project (includes detected `playbooks`) |
| DELETE | `/api/projects/{id}` | Delete project (and its files) |
| GET | `/api/projects/{id}/playbooks` | Detected playbook paths |
| GET | `/api/projects/{id}/files` | File tree |
| GET | `/api/projects/{id}/file?path=…` | Read a file `{path, content, size}` |
| PUT | `/api/projects/{id}/file` | Write `{path, content}` |

### Inventories
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects/{id}/inventories` | List inventories for a project |
| POST | `/api/projects/{id}/inventories` | Create `{name, content}` |
| GET | `/api/inventories/{id}` | Get |
| PUT | `/api/inventories/{id}` | Update `{name?, content?}` |
| DELETE | `/api/inventories/{id}` | Delete |

### Templates
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/templates[?projectId=]` | List templates |
| POST | `/api/templates` | Create (full template body) |
| GET/PUT/DELETE | `/api/templates/{id}` | Get / update / delete |
| POST | `/api/templates/{id}/run` | Launch from template (optional override body) → `Run` |

### Runs
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/runs[?projectId=&status=&limit=]` | List runs (newest first) |
| POST | `/api/runs` | Launch ad-hoc → `202` + `Run` |
| GET | `/api/runs/{id}` | `{run, active}` |
| GET | `/api/runs/{id}/output` | Stored raw terminal output (text/plain, with ANSI) |
| POST | `/api/runs/{id}/cancel` | Cancel a running run |

**Launch body** (`POST /api/runs`):
```json
{
  "projectId": "prj_…",
  "playbook": "playbooks/10-full-stack.yml",
  "inventoryId": "inv_…",        // optional; omit for the project default
  "limit": "web",                 // optional
  "tags": "deploy",               // optional
  "skipTags": "",                 // optional
  "extraVars": { "app_version": "2.0.0" },
  "check": false,                  // --check (dry run)
  "diff": true,                    // --diff
  "verbosity": 0                   // 0..4  →  -v..-vvvv
}
```

## WebSocket

### `GET /ws/events`
Server → client JSON messages whenever state changes:
```json
{ "type": "run.updated", "run": { … } }
{ "type": "project.created", "project": { … } }
```
The server sends periodic ping frames; reconnect with backoff on close.

### `GET /ws/runs/{id}` — the live terminal
- **Server → client**
  - **binary** frames: raw terminal bytes → feed straight into `xterm.write()`.
  - **text** frames (JSON): `{"type":"status","status":"running","run":{…}}` on connect and
    `{"type":"end","status":"success","run":{…}}` when the run finishes.
- **Client → server** (text JSON):
  - `{"type":"resize","cols":120,"rows":34}`
  - `{"type":"cancel"}`

Opening an **active** run replays the buffered output, then streams live. Opening a
**finished** run replays the stored output, then sends `end`.
