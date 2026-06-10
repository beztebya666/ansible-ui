const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const PROJ=process.env.PROJ||"";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:1050}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  const cap=async(lang)=>{
    await p.evaluate(l=>localStorage.setItem("aui.lang",l),lang);
    await p.goto(`${base}/projects/${PROJ}`,{waitUntil:"networkidle2"}); await s(1200);
    // switch to the Inventories tab
    await p.evaluate(()=>{const t=[...document.querySelectorAll("button,a,[role='tab']")].find(e=>/Инвентари|Inventories|Inventory/i.test(e.textContent||""));if(t)t.click();});
    await s(1000);
    const r=await p.evaluate(()=>{
      const lbl=[...document.querySelectorAll("*")].find(e=>e.textContent.trim()==="inv-edge"&&e.children.length===0);
      if(!lbl) return "no-label";
      let row=lbl;
      for(let i=0;i<6&&row;i++){const btn=[...row.querySelectorAll("button[title]")].find(x=>/edit|Изменить/i.test(x.title));if(btn){btn.click();return "clicked";}row=row.parentElement;}
      return "no-btn";
    });
    await s(1000);
    await p.evaluate(()=>{const i=document.querySelector('input[placeholder="edge-dc1"]');if(i)i.scrollIntoView({block:"center"});});
    await s(500);
    await p.screenshot({path:`/out/affinity-${lang}.png`}); console.log("shot",lang,r);
    await p.keyboard.press("Escape"); await s(400);
  };
  await cap("ru");
  await cap("en");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
