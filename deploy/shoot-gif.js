const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080"; const RID=process.env.RID||"";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1280,height:680}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(500);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(700);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","en"));
  await p.goto(`${base}/runs/${RID}`,{waitUntil:"networkidle2"}); await s(400);
  const clip={x:240,y:0,width:1040,height:680}; // right of the nav rail
  for(let i=0;i<15;i++){
    await p.screenshot({path:`/out/frames/frame-${String(i).padStart(2,"0")}.png`,clip});
    await s(1000);
  }
  console.log("captured 15 frames");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
