const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE,pid=process.env.PID; const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/projects/${pid}`,{waitUntil:"networkidle2"}); await s(1200);
  // click "Новый запуск"
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(b=>/Новый запуск/i.test(b.textContent||"")&&b.offsetParent!==null);b&&b.click();}); await s(1000);
  // open the INVENTORIES select (the one showing "По умолчанию для проекта")
  await p.evaluate(()=>{const sel=[...document.querySelectorAll("button")].find(b=>/По умолчанию для проекта/.test(b.textContent||""));sel&&sel.click();}); await s(700);
  await p.screenshot({path:"/out/inv-dropdown.png"}); console.log("shot inv-dropdown");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
