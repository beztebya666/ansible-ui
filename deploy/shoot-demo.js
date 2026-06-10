const puppeteer = require("puppeteer");
(async () => {
  const base = "http://localhost:8099";
  const b = await puppeteer.launch({ headless: "new", executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || "/usr/bin/chromium-browser", args: ["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage"] });
  const p = await b.newPage();
  await p.setViewport({ width: 1440, height: 1024 });
  const s = (ms) => new Promise((r) => setTimeout(r, ms));
  p.on("pageerror", (e) => console.log("PAGEERROR:", String(e).slice(0, 200)));
  p.on("console", (m) => { if (m.type() === "error") console.log("CONSOLE.ERR:", m.text().slice(0, 160)); });
  await p.goto(base + "/", { waitUntil: "networkidle2" });
  await s(2500);
  await p.screenshot({ path: "/out/demo-dashboard.png" });
  console.log("shot dashboard");
  const go = async (hash, name, wait = 1800) => {
    await p.evaluate((h) => { location.hash = h; }, hash);
    await s(wait);
    await p.screenshot({ path: `/out/${name}.png` });
    console.log("shot", name);
  };
  await go("#/runs", "demo-runs");
  await go("#/runs/run_live1", "demo-terminal", 7000); // let the live stream play
  await go("#/workflows", "demo-workflows");
  await go("#/insights", "demo-insights", 2200);
  await go("#/cluster", "demo-cluster");
  await go("#/templates", "demo-templates");
  await go("#/credentials", "demo-credentials");
  await b.close();
  console.log("DONE");
})().catch((e) => { console.error(e); process.exit(1); });
