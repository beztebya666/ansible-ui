const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1120,height:920}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/environments`,{waitUntil:"networkidle2"}); await s(900);
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Секрет|Хранилища секретов|менеджер/i.test(x.textContent));if(b)b.click();});
  await s(900);
  // edit gcp-test → GCP form (project + SA JSON textarea)
  await p.evaluate(()=>{const rows=[...document.querySelectorAll(".card .flex.items-center")];for(const r of rows){if(/gcp-test/.test(r.textContent)){const btn=r.querySelector("button");if(btn){btn.click();return;}}}});
  await s(700);
  await p.screenshot({path:"/out/gcp-form.png"}); console.log("gcp");
  // switch the type select to Azure to show azure fields (edit az-test instead)
  await p.evaluate(()=>{const rows=[...document.querySelectorAll(".card .flex.items-center")];for(const r of rows){if(/az-test/.test(r.textContent)){const btn=r.querySelector("button");if(btn){btn.click();return;}}}});
  await s(700);
  await p.screenshot({path:"/out/azure-form.png"}); console.log("azure");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
