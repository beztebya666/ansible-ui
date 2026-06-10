# Demo Ansible project

A self-contained, **localhost-safe** Ansible project that ships with ansible-ui.
Every playbook runs with `connection: local` and writes only under
`/tmp/ansible-ui-demo`, so you can run any of them repeatedly with no remote
hosts, credentials, or privileges.

## Playbooks (simple → advanced)

| # | Playbook | Teaches |
|---|----------|---------|
| 01 | `playbooks/01-hello.yml` | ping, debug, loops — your first green run |
| 02 | `playbooks/02-facts.yml` | fact gathering, `set_fact` |
| 03 | `playbooks/03-variables-loops.yml` | vars, loops, conditionals, `register`, `assert` |
| 04 | `playbooks/04-files-and-templates.yml` | `copy`, `template`, `lineinfile`, `blockinfile`, handlers, idempotency |
| 05 | `playbooks/05-handlers.yml` | `notify`, `listen`, `flush_handlers` |
| 06 | `playbooks/06-roles.yml` | role composition (`common`, `webserver`, `monitoring`) |
| 07 | `playbooks/07-blocks-rescue.yml` | `block`/`rescue`/`always`, soft failures |
| 08 | `playbooks/08-async-parallel.yml` | `async`/`poll`, `async_status` |
| 09 | `playbooks/09-vault.yml` | secrets, `no_log`, Ansible Vault |
| 10 | `playbooks/10-full-stack.yml` | pre/post tasks, tag-driven roles, verification gate |

## Run from the CLI

```bash
ansible-playbook playbooks/10-full-stack.yml
ansible-playbook playbooks/03-variables-loops.yml --check --diff
ansible-playbook playbooks/06-roles.yml --tags web -vv
```

## Vault

`vault/secrets.yml` ships as plaintext so the demo runs out of the box. To make
it realistic:

```bash
ansible-vault encrypt vault/secrets.yml   # uses the shipped .vault_pass
ansible-playbook playbooks/09-vault.yml   # decrypts automatically
```

> ⚠️ The bundled `.vault_pass` is for the demo only. Never commit real secrets.
