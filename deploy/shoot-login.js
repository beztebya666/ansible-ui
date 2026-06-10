const puppeteer = require("puppeteer");
(async () => {
  const b = await puppeteer.launch({ headless:"new", executablePath: process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser", args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"] });
  const p = await b.newPage(); await p.setViewport({width:520,height:760});
  await p.goto("http://localhost:8080", {waitUntil:"networkidle2"}); await new Promise(r=>setTimeout(r,1500));
  await p.screenshot({path:"/out/login-oauth.png"}); console.log("shot login");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
