// Screenshot the Add-notification-channel dialog with Email (SMTP) selected.
const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1100, height: 950 });
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
  await page.goto(`${base}/settings`, { waitUntil: "networkidle2" });
  await sleep(1200);
  // open the "Add channel" dialog
  await page.evaluate(() => {
    const b = [...document.querySelectorAll("button")].find((x) => /channel|канал/i.test(x.textContent || ""));
    b && b.click();
  });
  await sleep(700);
  // open the Type select and choose Email (SMTP)
  await page.evaluate(() => {
    const trigger = document.querySelector(".modal, [role=dialog]")?.querySelector("button");
    // click the first select trigger in the dialog
    const sel = [...document.querySelectorAll("button")].find((b) => /telegram/i.test(b.textContent || ""));
    sel && sel.click();
  });
  await sleep(400);
  await page.evaluate(() => {
    const opt = [...document.querySelectorAll("*")].find((e) => e.children.length === 0 && /Email \(SMTP\)/.test(e.textContent || ""));
    opt && opt.click();
  });
  await sleep(600);
  await page.screenshot({ path: "/out/email-form.png" });
  console.log("shot email-form");
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
