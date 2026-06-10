// Screenshot a Semaphore task view at the top AND scrolled to the bottom.
const puppeteer = require("puppeteer");
(async () => {
  const browser = await puppeteer.launch({
    headless: "new",
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1700, height: 1050 });
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  await page.goto("http://localhost:3000/", { waitUntil: "networkidle2" });
  await sleep(1500);
  if (await page.$('input[type="password"]')) {
    for (const inp of await page.$$("input")) {
      const ty = await (await inp.getProperty("type")).jsonValue();
      if (ty === "text" || ty === "email") { await inp.type("admin"); break; }
    }
    await page.type('input[type="password"]', "changeme");
    await page.keyboard.press("Enter");
    await sleep(2500);
  }
  await page.goto("http://localhost:3000" + process.env.SEM_VIEW, { waitUntil: "networkidle2" });
  await sleep(3500);
  // scroll the task log to the TOP first
  await page.evaluate(() => {
    const sc = [...document.querySelectorAll("*")].find((d) => d.scrollHeight > d.clientHeight + 80 && /auto|scroll/.test(getComputedStyle(d).overflowY));
    if (sc) sc.scrollTop = 0;
  });
  await sleep(600);
  await page.screenshot({ path: "/out/semfs-top.png" });
  console.log("shot semfs-top");
  await page.evaluate(() => {
    const sc = [...document.querySelectorAll("*")].find((d) => d.scrollHeight > d.clientHeight + 80 && /auto|scroll/.test(getComputedStyle(d).overflowY));
    if (sc) sc.scrollTop = sc.scrollHeight;
  });
  await sleep(600);
  await page.screenshot({ path: "/out/semfs-bottom.png" });
  console.log("shot semfs-bottom");
  await browser.close();
})().catch((e) => { console.error(e); process.exit(1); });
