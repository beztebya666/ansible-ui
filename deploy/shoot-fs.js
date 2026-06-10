// Open a project's Files tab, select a file, toggle the editor to fullscreen and screenshot.
const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const pid = process.env.PID;
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1500, height: 950 });
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
  await page.goto(`${base}/projects/${pid}?tab=files`, { waitUntil: "networkidle2" });
  await sleep(1800);
  // select a file by clicking the tree row whose trimmed text is exactly the name
  const picked = await page.evaluate(() => {
    const want = "01-hello.yml";
    const el = [...document.querySelectorAll("*")].find((e) => (e.textContent || "").trim() === want && e.querySelector("*") === null);
    if (!el) return false;
    (el.closest("button,[role=button],li,div") || el).click();
    return true;
  });
  console.log("file picked:", picked);
  await sleep(1500);
  // click the fullscreen (maximize) button in the editor header
  const clicked = await page.evaluate(() => {
    const btn = [...document.querySelectorAll("button")].find((b) => /fullscreen|во весь|развернуть/i.test(b.getAttribute("title") || ""));
    if (btn) { btn.click(); return true; }
    return false;
  });
  await sleep(900);
  await page.screenshot({ path: "/out/fs-editor.png" });
  console.log("fullscreen toggled:", clicked, "→ shot fs-editor");
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
