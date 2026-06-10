const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE,rd=process.env.RD;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:820}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  // templates list with Build/Deploy badges
  await p.goto(`${base}/templates`,{waitUntil:"networkidle2"}); await s(1400);
  await p.screenshot({path:"/out/tt-list.png"}); console.log("shot list");
  // open the deploy template's edit modal
  const clicked=await p.evaluate(()=>{
    const cards=[...document.querySelectorAll(".card")];
    for(const c of cards){const h=c.querySelector("h3");if(h&&h.textContent.trim()==="Ship it"){const btn=c.querySelector("button");if(btn){btn.click();return true;}}}
    return false;
  });
  await s(1200);
  await p.screenshot({path:"/out/tt-form.png"}); console.log("shot form clicked="+clicked);
  // deploy run detail (artifact row)
  await p.goto(`${base}/runs/${rd}`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/tt-run.png"}); console.log("shot run");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
