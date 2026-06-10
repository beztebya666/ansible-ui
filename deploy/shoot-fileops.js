const puppeteer = require("puppeteer");
(async () => {
  const base=process.env.BASE, pid=process.env.PID;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1100,height:760}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.goto(`${base}/projects/${pid}?tab=files`,{waitUntil:"networkidle2"}); await s(1500);
  // hover the 01-hello.yml row
  const box = await p.evaluate(()=>{
    const el=[...document.querySelectorAll("span")].find(e=>(e.textContent||"").trim()==="01-hello.yml");
    if(!el) return null; const row=el.closest("div"); const r=row.getBoundingClientRect();
    return {x:r.x+r.width/2,y:r.y+r.height/2};
  });
  if(box){ await p.mouse.move(box.x,box.y); await s(500); }
  await p.screenshot({path:"/out/fileops.png"}); console.log("hover box:",!!box,"shot fileops");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
