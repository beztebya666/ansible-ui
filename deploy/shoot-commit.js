const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",pid="prj_74eae67f23219974";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:840}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}?tab=files`,{waitUntil:"networkidle2"}); await s(1500);
  // click site.yml in the tree
  await p.evaluate(()=>{const el=[...document.querySelectorAll("*")].find(e=>e.children.length===0&&/site\.yml/.test(e.textContent||""));if(el)el.click();});
  await s(1200);
  // focus the code editor + type to make it dirty
  const ed=await p.$(".cm-content"); if(ed){await ed.click();await p.keyboard.type("  # edited in UI");}
  await s(500);
  // click "Коммит и пуш"
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Коммит и пуш/.test(x.textContent));if(b)b.click();});
  await s(1000);
  await p.screenshot({path:"/out/commit-dialog.png"}); console.log("shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
