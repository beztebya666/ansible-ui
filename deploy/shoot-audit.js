const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:860}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/audit`,{waitUntil:"networkidle2"}); await s(1300);
  await p.screenshot({path:"/out/audit-ru.png"}); console.log("shot audit");
  await p.goto(`${base}/`,{waitUntil:"networkidle2"}); await s(1300);
  await p.screenshot({path:"/out/dash-ru.png"}); console.log("shot dash");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
