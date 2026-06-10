// Baked, realistic ANSI output for demo runs — rendered in the live terminal and
// replayed for finished runs. Colours match ansible's default stdout callback.
/* eslint-disable no-control-regex */

const G = "[0;32m"; // ok (green)
const Y = "[0;33m"; // changed (yellow)
const R = "[0;31m"; // failed / unreachable (red)
const C = "[0;36m"; // skipped (cyan)
const B = "[1;34m"; // bright blue
const X = "[0m"; // reset

const stars = (s: string) => s + " " + "*".repeat(Math.max(4, 72 - s.length));

export interface AnsibleLogOpts {
  play: string;
  playbook: string;
  hosts: string[];
  tasks: { name: string; result: "ok" | "changed" | "skipped"; msg?: string }[];
  recap: { ok: number; changed: number; failed?: number; unreachable?: number };
  failTask?: string; // if set, the run fails on this task
  vv?: boolean;
}

/** Build a realistic ansible-playbook log (with -vv task paths + config banner). */
export function ansibleLog(o: AnsibleLogOpts): string {
  const L: string[] = [];
  const base = `/data/projects/demo/${o.playbook}`;
  if (o.vv) {
    L.push(`ansible-playbook [core 2.20.6]`);
    L.push(`  config file = /data/projects/demo/ansible.cfg`);
    L.push(`  python version = 3.12.7`);
    L.push(`  executable location = /usr/local/bin/ansible-playbook`);
    L.push("");
  }
  L.push(stars(`PLAYBOOK: ${o.playbook}`));
  L.push("");
  L.push(stars(`PLAY [${o.play}]`));
  L.push("");
  L.push(stars("TASK [Gathering Facts]"));
  if (o.vv) L.push(`task path: ${base}:1`);
  for (const h of o.hosts) L.push(`${G}ok: [${h}]${X}`);
  L.push("");
  let line = 5;
  for (const t of o.tasks) {
    L.push(stars(`TASK [${t.name}]`));
    if (o.vv) L.push(`task path: ${base}:${(line += 6)}`);
    if (o.failTask && t.name === o.failTask) {
      for (const h of o.hosts) {
        L.push(`${R}fatal: [${h}]: FAILED! => {"changed": false, "msg": "${t.msg || "task failed"}"}${X}`);
      }
      L.push("");
      L.push(stars("PLAY RECAP"));
      for (const h of o.hosts)
        L.push(
          `${R}${h}${X} : ${G}ok=${Math.max(1, line / 6 | 0)}${X} ${Y}changed=0${X} unreachable=0 ${R}failed=1${X} skipped=0 rescued=0 ignored=0`,
        );
      return L.join("\r\n") + "\r\n";
    }
    const col = t.result === "ok" ? G : t.result === "changed" ? Y : C;
    const verb = t.result === "skipped" ? "skipping" : t.result;
    for (const h of o.hosts) {
      if (t.msg && t.result !== "skipped") {
        L.push(`${col}${t.result}: [${h}] => {${X}`);
        L.push(`${col}    "msg": "${t.msg}"${X}`);
        L.push(`${col}}${X}`);
      } else {
        L.push(`${col}${verb}: [${h}]${X}`);
      }
    }
    L.push("");
  }
  L.push(stars("PLAY RECAP"));
  for (const h of o.hosts) {
    L.push(
      `${G}${h}${X} : ${G}ok=${o.recap.ok}${X} ${Y}changed=${o.recap.changed}${X} unreachable=${o.recap.unreachable || 0} failed=${o.recap.failed || 0} skipped=0 rescued=0 ignored=0`,
    );
  }
  return L.join("\r\n") + "\r\n";
}

export function terraformLog(): string {
  return [
    stars("PLAYBOOK: terraform apply"),
    "",
    `${B}Initializing the backend...${X}`,
    `${B}Initializing provider plugins...${X}`,
    `- Finding hashicorp/aws versions matching "~> 5.0"...`,
    `- Installed hashicorp/aws v5.31.0`,
    "",
    `${G}Terraform has been successfully initialized!${X}`,
    "",
    `Terraform used the selected providers to generate the following execution plan.`,
    `Resource actions are indicated with the following symbols:`,
    `  ${G}+ create${X}`,
    "",
    `Terraform will perform the following actions:`,
    "",
    `  ${G}# aws_instance.web[0] will be created${X}`,
    `  ${G}+${X} resource "aws_instance" "web" {`,
    `      ${G}+${X} ami           = "ami-0c55b159cbfafe1f0"`,
    `      ${G}+${X} instance_type = "t3.micro"`,
    `      ${G}+${X} tags          = { "Name" = "web-01" }`,
    `    }`,
    "",
    `${B}Plan:${X} ${G}3 to add${X}, 0 to change, 0 to destroy.`,
    "",
    `aws_instance.web[0]: Creating...`,
    `aws_instance.web[0]: ${G}Creation complete after 22s${X} [id=i-0a1b2c3d4e5f]`,
    "",
    `${G}Apply complete! Resources: 3 added, 0 changed, 0 destroyed.${X}`,
    "",
  ].join("\r\n") + "\r\n";
}

export function bashLog(): string {
  return [
    stars("PLAYBOOK: healthcheck.sh"),
    "",
    `${B}==> running health checks across the fleet${X}`,
    `  web-01.prod ... ${G}OK${X} (200, 41ms)`,
    `  web-02.prod ... ${G}OK${X} (200, 38ms)`,
    `  db-01.prod  ... ${G}OK${X} (pg_isready)`,
    `  cache-01    ... ${G}OK${X} (PONG)`,
    "",
    `${G}✓ all 4 services healthy${X}`,
    "",
  ].join("\r\n") + "\r\n";
}

/** A curated catalogue of finished-run logs the seed reuses. */
export const LOGS: Record<string, () => string> = {
  deploy: () =>
    ansibleLog({
      play: "Deploy web tier",
      playbook: "deploy.yml",
      hosts: ["web-01.prod", "web-02.prod"],
      vv: true,
      tasks: [
        { name: "Pull application artifact", result: "changed", msg: "fetched build #482" },
        { name: "Render config from template", result: "changed" },
        { name: "Install systemd unit", result: "ok" },
        { name: "Restart application", result: "changed" },
        { name: "Wait for health endpoint", result: "ok", msg: "✓ /healthz returned 200" },
        { name: "Smoke test", result: "ok", msg: "✓ app v1.4.2 deployed" },
      ],
      recap: { ok: 8, changed: 3 },
    }),
  facts: () =>
    ansibleLog({
      play: "Gather & assert facts",
      playbook: "facts.yml",
      hosts: ["web-01.prod", "web-02.prod", "db-01.prod"],
      tasks: [
        { name: "Collect distribution", result: "ok", msg: "Ubuntu 22.04" },
        { name: "Assert minimum memory", result: "ok" },
        { name: "Set custom fact", result: "changed" },
      ],
      recap: { ok: 5, changed: 1 },
    }),
  failed: () =>
    ansibleLog({
      play: "Deploy web tier",
      playbook: "deploy.yml",
      hosts: ["web-03.staging"],
      vv: true,
      tasks: [
        { name: "Pull application artifact", result: "changed" },
        { name: "Wait for health endpoint", result: "ok", msg: "health endpoint returned 503 after 5 retries" },
      ],
      failTask: "Wait for health endpoint",
      recap: { ok: 2, changed: 1, failed: 1 },
    }),
  terraform: terraformLog,
  bash: bashLog,
};
