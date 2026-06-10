const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080", pid = process.env.PID;
  const b = await puppeteer.launch({ headless:"new", executablePath: process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser", args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"] });
  const p = await b.newPage(); await p.setViewport({width:1300,height:760});
  const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if (await p.$('input[type="password"]')) { const u=await p.$('input[autocomplete="username"]'); if(u)await u.type("admin"); await p.type('input[type="password"]',"admin123"); await p.click("button.btn-primary"); await p.waitForSelector("aside",{timeout:10000}).catch(()=>{}); await s(900);}
  await p.goto(`${base}/projects/${pid}?tab=files`,{waitUntil:"networkidle2"}); await s(1600);
  await p.screenshot({path:"/out/files-upload.png"}); console.log("shot files-upload");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
