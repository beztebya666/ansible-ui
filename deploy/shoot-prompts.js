// Open the Templates page, click Run on the full-stack template, and screenshot
// the launch-time prompts dialog.
const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1400, height: 1000 });
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
  await page.goto(`${base}/templates`, { waitUntil: "networkidle2" });
  await sleep(1500);

  // Find the card mentioning 10-full-stack and click its Run button.
  const clicked = await page.evaluate(() => {
    const card = [...document.querySelectorAll(".card")].find((c) => /10-full-stack/.test(c.textContent || ""));
    if (!card) return "no-card";
    const btn = [...card.querySelectorAll("button")].find((b) => /run|запуст/i.test(b.textContent || ""));
    if (!btn) return "no-btn";
    btn.click();
    return "ok";
  });
  console.log("run click:", clicked);
  await sleep(1500);
  await page.screenshot({ path: "/out/prompts-dialog.png" });
  console.log("shot prompts-dialog");
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
