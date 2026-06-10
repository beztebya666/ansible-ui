// Screenshot the run console in both Terminal and the new structured Log view.
const puppeteer = require("puppeteer");

(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const runId = process.env.RUN_ID;
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
  const hasPassword = await page.$('input[type="password"]');
  if (hasPassword) {
    const user = await page.$('input[autocomplete="username"]');
    if (user) await user.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1200);
  }

  await page.goto(`${base}/runs/${runId}`, { waitUntil: "networkidle2" });
  await sleep(2500);
  await page.screenshot({ path: "/out/run-terminal.png" });
  console.log("shot run-terminal");

  // Click the "Log" tab (title is "Log" in EN, "Журнал" in RU).
  const clicked = await page.evaluate(() => {
    const b = [...document.querySelectorAll("button")].find((x) => /^(Log|Журнал)$/.test(x.getAttribute("title") || ""));
    if (b) { b.click(); return true; }
    return false;
  });
  console.log("log tab clicked:", clicked);
  await sleep(2000);
  await page.screenshot({ path: "/out/run-log.png" });
  console.log("shot run-log");

  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
