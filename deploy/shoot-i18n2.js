const puppeteer = require("puppeteer");
(async () => {
  const base = process.env.BASE || "http://localhost:8080";
  const b = await puppeteer.launch({ headless:"new", executablePath: process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser", args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"] });
  const p = await b.newPage(); await p.setViewport({width:1280,height:1050});
  const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if (await p.$('input[type="password"]')) { const u=await p.$('input[autocomplete="username"]'); if(u)await u.type("admin"); await p.type('input[type="password"]',"admin123"); await p.click("button.btn-primary"); await p.waitForSelector("aside",{timeout:10000}).catch(()=>{}); await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  // Template editor
  await p.goto(`${base}/templates`,{waitUntil:"networkidle2"}); await s(1200);
  await p.evaluate(()=>{const btn=[...document.querySelectorAll("button.btn-primary")].find(b=>b.offsetParent!==null);btn&&btn.click();});
  await s(1000);
  await p.screenshot({path:"/out/i18n-template-form.png"}); console.log("shot template-form");
  // Launcher (global New run)
  await p.goto(`${base}/runs`,{waitUntil:"networkidle2"}); await s(1000);
  await p.evaluate(()=>{const btn=[...document.querySelectorAll("button")].find(b=>/Новый запуск|New run/i.test(b.textContent||""));btn&&btn.click();});
  await s(1000);
  await p.screenshot({path:"/out/i18n-launcher.png"}); console.log("shot launcher");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
