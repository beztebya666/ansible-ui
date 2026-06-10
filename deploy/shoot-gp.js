const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",pid="prj_11b20c04b8934569";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:880}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}?tab=files`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/gp-branches.png"}); console.log("branches shot");
  // open a.yml, edit, open commit dialog (shows diff)
  await p.evaluate(()=>{const el=[...document.querySelectorAll("*")].find(e=>e.children.length===0&&/^a\.yml$/.test((e.textContent||"").trim()));if(el)el.click();});
  await s(1000);
  const ed=await p.$(".cm-content"); if(ed){await ed.click();await p.keyboard.type("  # changed in UI");}
  await s(400);
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Коммит и пуш/.test(x.textContent));if(b)b.click();});
  await s(900);
  // expand the diff
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/a\.yml/.test(x.textContent)&&x.querySelector("svg"));if(b)b.click();});
  await s(500);
  await p.screenshot({path:"/out/gp-diff.png"}); console.log("diff shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
