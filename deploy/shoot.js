// Headless screenshot helper: logs in, then captures the key pages.
const puppeteer = require("puppeteer");

(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1600, height: 1000 });
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  await page.goto(base, { waitUntil: "networkidle2" });
  await sleep(1200);

  // Log in if the auth form is showing.
  const hasPassword = await page.$('input[type="password"]');
  if (hasPassword) {
    const user = await page.$('input[autocomplete="username"]');
    if (user) await user.type("admin");
    await page.type('input[type="password"]', "admin123");
    await page.screenshot({ path: "/out/login.png" });
    await page.click("button.btn-primary");
    await page.waitForSelector("aside", { timeout: 10000 }).catch(() => {});
    await sleep(1500);
  }

  const shots = [
    ["dashboard", "/"],
    ["repositories", "/repositories"],
    ["environments", "/environments"],
    ["credentials", "/credentials"],
    ["templates", "/templates"],
    ["settings", "/settings"],
  ];
  for (const [name, path] of shots) {
    await page.goto(base + path, { waitUntil: "networkidle2" });
    await sleep(1400);
    await page.screenshot({ path: "/out/" + name + ".png" });
    console.log("shot", name);
  }
  await browser.close();
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
