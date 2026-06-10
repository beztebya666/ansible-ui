const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:760}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(500);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(700);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs`,{waitUntil:"networkidle2"}); await s(1000);
  await p.screenshot({path:"/out/cc-runs.png"}); console.log("shot runs");
  await p.goto(`${base}/runners`,{waitUntil:"networkidle2"}); await s(900);
  await p.screenshot({path:"/out/cc-runners.png"}); console.log("shot runners");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
