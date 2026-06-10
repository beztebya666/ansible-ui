// Verify the new console: screenshot our build.yml run (galaxy spinner + banners)
// and a fresh executing run (colours), scrolled to the top.
const puppeteer = require("puppeteer");

(async () => {
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1700, height: 1100 });
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  await page.goto("http://localhost:8080", { waitUntil: "networkidle2" });
  await sleep(1000);
  const pw = await page.$('input[type="password"]');
  if (pw) {
    const u = await page.$('input[autocomplete="username"]');
    if (u) await u.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1200);
  }

  const shoot = async (runId, name) => {
    if (!runId) return;
    await page.goto("http://localhost:8080/runs/" + runId, { waitUntil: "networkidle2" });
    await sleep(2500);
    // scroll the log container to the top so we see the galaxy/spinner area
    await page.evaluate(() => {
      const el = document.querySelector(".overflow-y-auto");
      if (el) el.scrollTop = 0;
    });
    await sleep(600);
    await page.screenshot({ path: "/out/" + name + ".png" });
    console.log("shot", name, runId);
  };

  await shoot(process.env.RUN_BUILD, "verify-build");
  await shoot(process.env.RUN_HELLO, "verify-hello");
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
