const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:860}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/api`,{waitUntil:"networkidle2"}); await s(1500);
  // try to click the Applications group if it's collapsible
  await p.evaluate(()=>{const el=[...document.querySelectorAll("*")].find(e=>e.textContent.trim()==="Applications"&&e.children.length===0);if(el&&el.click)el.click();});
  await s(700);
  await p.screenshot({path:"/out/api-explorer.png"}); console.log("shot api");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
