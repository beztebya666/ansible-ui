// Verify the run log appears live (no manual refresh) when a run finishes while
// the run-detail page is open. Launches a fresh run, navigates to it, then waits
// WITHOUT reloading and checks the log filled in by itself.
const puppeteer = require("puppeteer");

(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1700, height: 1000 });
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  await page.goto(base, { waitUntil: "networkidle2" });
  await sleep(1000);
  if (await page.$('input[type="password"]')) {
    const user = await page.$('input[autocomplete="username"]');
    if (user) await user.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1200);
  }

  // Launch a fresh run via the in-page session, then immediately open it. The run
  // is pending/running at this point, so the page mounts in the live state and we
  // must NOT reload — the log has to populate on its own.
  const runId = await page.evaluate(async () => {
    const ts = await fetch("/api/templates", { credentials: "include" }).then((r) => r.json());
    const tpl = ts.find((t) => (t.app || "ansible") === "ansible") || ts[0];
    const run = await fetch(`/api/templates/${tpl.id}/run`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: "{}",
    }).then((r) => r.json());
    return run.id;
  });
  console.log("launched run:", runId);

  await page.goto(`${base}/runs/${runId}`, { waitUntil: "domcontentloaded" });
  // Wait for the run to finish + the live update to land — NO reload in between.
  await sleep(9000);

  const body = await page.evaluate(() => document.body.innerText);
  const empty = /no output|нет вывода|—\s*no output\s*—/i.test(body);
  const hasRecap = /PLAY RECAP/i.test(body);
  console.log("log empty (bad):", empty, "| has PLAY RECAP (good):", hasRecap);

  await page.screenshot({ path: "/out/run-live.png" });
  console.log("shot run-live");
  await browser.close();
  if (empty || !hasRecap) process.exit(2); // surface the failure to the shell
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
