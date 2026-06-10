const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1100,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/environments`,{waitUntil:"networkidle2"}); await s(1000);
  // open the "secret backends" modal
  const opened=await p.evaluate(()=>{const btns=[...document.querySelectorAll("button")];const b=btns.find(x=>/Секрет|Бэкенд|backend|Vault|менеджер/i.test(x.textContent));if(b){b.click();return b.textContent.trim();}return null;});
  await s(900);
  // click edit (pencil) on the aws-test row to load AWS fields
  const edited=await p.evaluate(()=>{const rows=[...document.querySelectorAll(".card .flex.items-center")];for(const r of rows){if(/aws-test/.test(r.textContent)){const btn=r.querySelector("button");if(btn){btn.click();return true;}}}return false;});
  await s(900);
  await p.screenshot({path:"/out/aws-backend.png"}); console.log("shot aws opened="+opened+" edited="+edited);
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
