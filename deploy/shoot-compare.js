// Screenshot Semaphore's task view and our run console, for a direct comparison.
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

  // ---- OURS ----
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
  await page.goto("http://localhost:8080/runs/" + (process.env.RUN_ID || "run_753c24ba2f3e7bd9"), { waitUntil: "networkidle2" });
  await sleep(3000);
  await page.screenshot({ path: "/out/cmp-ours.png" });
  console.log("shot cmp-ours");

  // ---- SEMAPHORE ----
  await page.goto("http://localhost:3000/", { waitUntil: "networkidle2" });
  await sleep(1500);
  const semPw = await page.$('input[type="password"]');
  if (semPw) {
    // Semaphore login: first text/email-ish field = username, then password.
    const inputs = await page.$$('input');
    for (const inp of inputs) {
      const type = await (await inp.getProperty("type")).jsonValue();
      if (type === "text" || type === "email") { await inp.type("admin"); break; }
    }
    await page.type('input[type="password"]', "changeme");
    await page.keyboard.press("Enter");
    await sleep(2500);
  }
  await page.goto("http://localhost:3000/project/1/templates/9/tasks?t=2147483645", { waitUntil: "networkidle2" });
  await sleep(3500);
  await page.screenshot({ path: "/out/cmp-semaphore.png" });
  const title = await page.title().catch(() => "");
  const hasLogin = !!(await page.$('input[type="password"]'));
  console.log("semaphore title:", title, "loginWall:", hasLogin);
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
