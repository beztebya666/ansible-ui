const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900,deviceScaleFactor:2}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/templates`,{waitUntil:"networkidle2"}); await s(1200);
  // filter to "-app"
  const sb=await p.$('input[placeholder*="Поиск"], input[type="search"], input'); if(sb){await sb.click();await sb.type("-app");}
  await s(900);
  await p.screenshot({path:"/out/bd-list.png",clip:{x:250,y:120,width:1040,height:360}}); console.log("list");
  // click the pencil (edit) inside the deploy-app card
  const ok=await p.evaluate(()=>{
    const cards=[...document.querySelectorAll("div")].filter(d=>/deploy-app/.test(d.textContent||"")&&d.querySelector('button[title]'));
    cards.sort((a,b)=>a.textContent.length-b.textContent.length);
    const card=cards[0]; if(!card)return false;
    const pencil=[...card.querySelectorAll("button")].find(btn=>btn.querySelector("svg")&&!/Запустить|Удал/.test(btn.textContent));
    if(pencil){pencil.click();return true;} return false;
  });
  await s(1200);
  await p.screenshot({path:"/out/bd-form.png"}); console.log("form ok="+ok);
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
