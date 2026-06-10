const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",pid="prj_05d201695d1f3de5",inv="inv_03c05ddf068e29dd";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1120,height:920}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}?tab=inventories`,{waitUntil:"networkidle2"}); await s(1300);
  // find the ec2-prod row by its CLOUD badge text, click its first action button (edit pencil)
  const ok=await p.evaluate(()=>{
    const rows=[...document.querySelectorAll(".card > div, .divide-y > div, div")].filter(d=>/ec2-prod/.test(d.textContent||"")&&[...d.querySelectorAll("button")].length>=2&&d.querySelectorAll("button").length<5);
    // pick the tightest row
    rows.sort((a,b)=>a.textContent.length-b.textContent.length);
    const row=rows[0]; if(!row)return false;
    const btn=row.querySelector("button"); if(btn){btn.click();return true;}
    return false;
  });
  await s(1100);
  await p.screenshot({path:"/out/cloud-form.png"}); console.log("shot ok="+ok);
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
