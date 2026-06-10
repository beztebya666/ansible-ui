// Screenshot the multi-app + new UI at desktop and mobile widths.
const puppeteer = require("puppeteer");

(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const shot = async (name) => { await page.screenshot({ path: "/out/" + name + ".png" }); console.log("shot", name); };

  const clickText = async (sel, text) => {
    const ok = await page.evaluate((sel, text) => {
      const els = Array.from(document.querySelectorAll(sel));
      const el = els.find((e) => (e.textContent || "").trim().toLowerCase().includes(text.toLowerCase()));
      if (el) { el.click(); return true; }
      return false;
    }, sel, text);
    return ok;
  };

  await page.setViewport({ width: 1560, height: 1000 });
  await page.goto(base, { waitUntil: "networkidle2" });
  await sleep(1000);

  // Login if needed.
  if (await page.$('input[type="password"]')) {
    const u = await page.$('input[autocomplete="username"]');
    if (u) await u.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1500);
  }

  // Desktop pages.
  for (const [name, path] of [
    ["dashboard", "/"],
    ["templates", "/templates"],
    ["runs", "/runs"],
    ["environments", "/environments"],
    ["settings", "/settings"],
  ]) {
    await page.goto(base + path, { waitUntil: "networkidle2" });
    await sleep(1300);
    await shot(name);
  }

  // Template form with the app picker (Ansible then Terraform).
  await page.goto(base + "/templates", { waitUntil: "networkidle2" });
  await sleep(1000);
  if (await clickText("button", "New template")) {
    await sleep(900);
    await shot("template-form");
    if (await clickText("button", "Terraform")) { await sleep(500); await shot("template-form-terraform"); }
    if (await clickText("button", "Bash")) { await sleep(500); await shot("template-form-bash"); }
    await page.keyboard.press("Escape");
    await sleep(400);
  }

  // Launcher (ad-hoc run) with the app picker.
  await page.goto(base + "/runs", { waitUntil: "networkidle2" });
  await sleep(900);
  if (await clickText("button", "New run")) {
    await sleep(900);
    await shot("launcher");
    await page.keyboard.press("Escape");
  }

  // Environment form with the Secrets editor.
  await page.goto(base + "/environments", { waitUntil: "networkidle2" });
  await sleep(900);
  if (await clickText("button", "New environment")) {
    await sleep(700);
    if (await clickText("button", "Add secret")) await sleep(300);
    await shot("environment-secrets");
    await page.keyboard.press("Escape");
  }

  // Applications (configurable backends).
  await page.goto(base + "/applications", { waitUntil: "networkidle2" });
  await sleep(1000);
  await shot("applications");

  // API Explorer (+ click an endpoint to show detail/try-it).
  await page.goto(base + "/api", { waitUntil: "networkidle2" });
  await sleep(1100);
  const epBtn = await page.$('main button');
  if (epBtn) { await epBtn.click(); await sleep(500); }
  await shot("api-explorer");

  // Dashboard Activity tab.
  await page.goto(base + "/", { waitUntil: "networkidle2" });
  await sleep(900);
  if (await clickText("button", "activity")) { await sleep(900); await shot("dashboard-activity"); }

  // Integration form with auth methods.
  await page.goto(base + "/integrations", { waitUntil: "networkidle2" });
  await sleep(700);
  if (await clickText("button", "New webhook")) {
    await sleep(700);
    await page.evaluate(() => {
      const sel = document.querySelector("select");
      // pick the auth <select> (second select in the modal) → GitHub
    });
    await shot("integration-form");
    await page.keyboard.press("Escape");
  }

  // Schedule "run once" form.
  await page.goto(base + "/schedules", { waitUntil: "networkidle2" });
  await sleep(700);
  if (await clickText("button", "New schedule")) {
    await sleep(600);
    if (await clickText("button", "Run once")) await sleep(400);
    await shot("schedule-once");
    await page.keyboard.press("Escape");
  }

  // Project detail: inventories tab (types) + files tab (syntax editor).
  await page.goto(base + "/projects", { waitUntil: "networkidle2" });
  await sleep(900);
  const card = (await page.$("a[href^='/projects/']")) || (await page.$("a.card"));
  if (card) {
    await card.click();
    await sleep(1200);
    if (await clickText("button", "inventories")) { await sleep(700); await shot("project-inventories"); }
    if (await clickText("button", "files")) {
      await sleep(900);
      const file = await page.$("ul li button");
      if (file) { await file.click(); await sleep(900); }
      await shot("project-files-syntax");
    }
  }

  // Project switcher (open) — the styled dropdown in the sidebar.
  await page.goto(base + "/templates", { waitUntil: "networkidle2" });
  await sleep(800);
  const sw = await page.$('aside button[aria-haspopup="listbox"]');
  if (sw) { await sw.click(); await sleep(500); await shot("switcher-open"); await page.keyboard.press("Escape"); }

  // A styled dropdown open on the Runs filters (proves no native <select>).
  await page.goto(base + "/runs", { waitUntil: "networkidle2" });
  await sleep(800);
  const dd = await page.$('main button[aria-haspopup="listbox"]');
  if (dd) { await dd.click(); await sleep(500); await shot("dropdown-open"); await page.keyboard.press("Escape"); }

  // The previously-broken failed run — header should now read "exit N · no recap".
  await page.goto(base + "/runs/run_ac2059f5a1a2d917", { waitUntil: "networkidle2" });
  await sleep(1500);
  await shot("failed-run-header");

  // Light theme: flip via the sidebar toggle (sun/moon is the first footer button).
  await page.goto(base + "/templates", { waitUntil: "networkidle2" });
  await sleep(800);
  await page.evaluate(() => {
    try { localStorage.setItem("aui.theme", "light"); document.documentElement.dataset.theme = "light"; } catch (e) {}
  });
  await sleep(600);
  await shot("templates-light");
  await page.evaluate(() => { try { localStorage.setItem("aui.theme", "dark"); document.documentElement.dataset.theme = "dark"; } catch (e) {} });

  // Mobile widths (responsive).
  await page.setViewport({ width: 390, height: 844 });
  for (const [name, path] of [["m-dashboard", "/"], ["m-templates", "/templates"], ["m-runs", "/runs"]]) {
    await page.goto(base + path, { waitUntil: "networkidle2" });
    await sleep(1100);
    await shot(name);
  }

  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
