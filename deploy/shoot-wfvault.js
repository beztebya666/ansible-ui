const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const PROJ=process.env.PROJ||"";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1320,height:1000}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  const cap=async(lang)=>{
    await p.evaluate((l,pr)=>{localStorage.setItem("aui.lang",l);localStorage.setItem("aui.project",pr);},lang,PROJ);
    await p.goto(`${base}/workflows`,{waitUntil:"networkidle2"}); await s(1300);
    const r=await p.evaluate(()=>{
      const lbl=[...document.querySelectorAll("*")].find(e=>/wf-vault-test/.test(e.textContent||"")&&e.children.length<2);
      if(!lbl) return "no-label";
      let row=lbl;
      for(let i=0;i<6&&row;i++){const btn=[...row.querySelectorAll("button[title]")].find(x=>/edit|Изменить/i.test(x.title));if(btn){btn.click();return "clicked";}row=row.parentElement;}
      return "no-btn";
    });
    await s(1100);
    await p.screenshot({path:`/out/wfvault-${lang}.png`}); console.log("shot",lang,r);
    await p.keyboard.press("Escape"); await s(400);
  };
  await cap("ru");
  await cap("en");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
