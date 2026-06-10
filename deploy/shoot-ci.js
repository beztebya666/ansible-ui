const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",pid="prj_05d201695d1f3de5";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1120,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}?tab=inventories`,{waitUntil:"networkidle2"}); await s(1300);
  // click edit (pencil) on ec2-prod
  await p.evaluate(()=>{const rows=[...document.querySelectorAll("*")].filter(e=>/ec2-prod/.test(e.textContent||""));for(const r of rows){const btn=r.querySelector&&r.querySelector("button");}const card=[...document.querySelectorAll("div")].find(d=>/ec2-prod/.test(d.textContent||"")&&d.querySelector("button"));if(card){const btns=[...card.querySelectorAll("button")];const edit=btns.find(b=>b.querySelector("svg"));if(edit)edit.click();}});
  await s(1000);
  await p.screenshot({path:"/out/cloud-inv.png"}); console.log("shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
