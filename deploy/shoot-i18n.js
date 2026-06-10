// Verify the admin forms are fully Russian (no English under RU). Logs in, flips
// the language to ru, then screenshots each page's list + its create/edit form.
const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 1000 });
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  await page.goto(base, { waitUntil: "networkidle2" });
  await sleep(800);
  if (await page.$('input[type="password"]')) {
    const u = await page.$('input[autocomplete="username"]');
    if (u) await u.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1000);
  }
  await page.evaluate(() => localStorage.setItem("aui.lang", "ru"));

  for (const route of ["integrations", "applications"]) {
    await page.goto(`${base}/${route}`, { waitUntil: "networkidle2" });
    await sleep(1200);
    await page.screenshot({ path: `/out/i18n-${route}-list.png` });
    // open the create form (header primary button)
    const opened = await page.evaluate(() => {
      const btn = [...document.querySelectorAll("button.btn-primary")].find((b) => b.offsetParent !== null);
      if (btn) { btn.click(); return true; }
      return false;
    });
    await sleep(900);
    await page.screenshot({ path: `/out/i18n-${route}-form.png` });
    console.log(route, "list+form, openedForm:", opened);
  }
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
