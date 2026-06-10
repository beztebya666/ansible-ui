const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:920}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/templates`,{waitUntil:"networkidle2"}); await s(1400);
  await p.screenshot({path:"/out/bd-list.png"}); console.log("list");
  // open the deploy-app template form
  await p.evaluate(()=>{const cards=[...document.querySelectorAll("a,div,button")].filter(e=>/deploy-app/.test(e.textContent||""));cards.sort((a,b)=>a.textContent.length-b.textContent.length);if(cards[0])cards[0].click();});
  await s(1200);
  await p.screenshot({path:"/out/bd-form.png"}); console.log("form");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
