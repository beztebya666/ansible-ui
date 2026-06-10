const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",pid="prj_05d201695d1f3de5",did="tpl_121d1ce36747cd7d";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}?tab=templates`,{waitUntil:"networkidle2"}); await s(1400);
  await p.screenshot({path:"/out/bd-list.png",clip:{x:250,y:120,width:1040,height:560}}); console.log("list");
  // open the deploy template to show the type selector + build picker
  await p.evaluate(()=>{const row=[...document.querySelectorAll("*")].find(e=>/deploy-app/.test(e.textContent||"")&&e.querySelector&&e.querySelector("button"));});
  await p.evaluate((did)=>{const cards=[...document.querySelectorAll("a,div,button")].filter(e=>/deploy-app/.test(e.textContent||""));cards.sort((a,b)=>a.textContent.length-b.textContent.length);if(cards[0])cards[0].click();},did);
  await s(1200);
  await p.screenshot({path:"/out/bd-form.png"}); console.log("form");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
