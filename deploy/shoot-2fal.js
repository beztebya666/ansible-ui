const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage","--incognito"]});
  const s=ms=>new Promise(r=>setTimeout(r,ms));
  const p=await b.newPage(); await p.setViewport({width:900,height:780,deviceScaleFactor:2});
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.reload({waitUntil:"networkidle2"}); await s(700);
  const u=await p.$('input[autocomplete="username"]');
  if(!u){console.log("no login form (already authed?)");}
  else { await u.type("tfa-user"); await p.type('input[type="password"]',"tfapass123"); await p.click("button.btn-primary"); await s(1500); }
  await p.screenshot({path:"/out/2fa-login.png"}); console.log("shot url="+p.url());
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
