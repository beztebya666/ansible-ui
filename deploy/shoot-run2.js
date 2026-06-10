// Screenshot a finished run's log at the top AND scrolled to the bottom, so we
// can inspect both the (verbose) header and the tail (recap / box-drawing).
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
  await sleep(800);
  if (await page.$('input[type="password"]')) {
    const u = await page.$('input[autocomplete="username"]');
    if (u) await u.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1000);
  }
  await page.goto(`${base}/runs/${runId}`, { waitUntil: "networkidle2" });
  await sleep(2500);
  await page.screenshot({ path: "/out/r2-top.png" });
  console.log("shot r2-top");

  // Scroll the log pane to the bottom.
  await page.evaluate(() => {
    const sc = [...document.querySelectorAll("div")].find(
      (d) => d.scrollHeight > d.clientHeight + 40 && /overflow-y-auto/.test(d.className),
    );
    if (sc) sc.scrollTop = sc.scrollHeight;
  });
  await sleep(800);
  await page.screenshot({ path: "/out/r2-bottom.png" });
  console.log("shot r2-bottom");

  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
