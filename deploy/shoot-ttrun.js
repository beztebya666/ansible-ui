const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE,rd=process.env.RD;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1500,height:900,deviceScaleFactor:1}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rd}`,{waitUntil:"networkidle2"}); await s(1600);
  // screenshot only the right config column (find the section listing rows)
  await p.screenshot({path:"/out/tt-run2.png",clip:{x:1000,y:60,width:500,height:520}});
  console.log("shot run2");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
