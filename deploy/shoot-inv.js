const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE,pid=process.env.PID; const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  // Inventories tab (detected)
  await p.goto(`${base}/projects/${pid}?tab=inventories`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/inv-detected.png"}); console.log("inv-detected");
  // Open the New inventory form, switch type to file → picker
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(b=>/Новый инвентарь/i.test(b.textContent||"")&&b.offsetParent!==null);b&&b.click();}); await s(800);
  // click the Type select then choose "Файл"
  await p.evaluate(()=>{const sels=[...document.querySelectorAll("button")].filter(b=>/Статический/i.test(b.textContent||""));sels[0]&&sels[0].click();}); await s(400);
  await p.evaluate(()=>{const o=[...document.querySelectorAll("*")].find(e=>e.children.length===0&&/Файл \(путь/.test(e.textContent||""));o&&o.click();}); await s(600);
  await p.screenshot({path:"/out/inv-form.png"}); console.log("inv-form");
  // Files tab search
  await p.goto(`${base}/projects/${pid}?tab=files`,{waitUntil:"networkidle2"}); await s(1500);
  await p.evaluate(()=>{const i=document.querySelector('input[placeholder*="Поиск"]');if(i){i.focus();i.value="inv";i.dispatchEvent(new Event("input",{bubbles:true}));const n=Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype,"value").set;}});
  // React controlled input — type via puppeteer
  const si=await p.$('input[placeholder*="Поиск"]'); if(si){await si.click({clickCount:3}); await si.type("inv");}
  await s(800); await p.screenshot({path:"/out/files-search.png"}); console.log("files-search");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
