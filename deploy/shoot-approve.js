const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_0f374d8440ec7660";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:760}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/approve.png",clip:{x:240,y:60,width:1060,height:120}}); console.log("shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
