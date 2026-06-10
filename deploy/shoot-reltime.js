const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE; const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:850}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  // start in EN
  await p.evaluate(()=>localStorage.setItem("aui.lang","en"));
  await p.goto(`${base}/runs`,{waitUntil:"networkidle2"}); await s(1500);
  // helper: extract relative-time tokens from the page
  const rel=()=>p.evaluate(()=>{const m=(document.body.innerText||"").match(/\d+\s*(?:[smhd]\s*ago|[смчдн]+\s*назад)|just now|только что/gi);return m?[...new Set(m)].slice(0,4):[];});
  const clickLang=(label)=>p.evaluate((lbl)=>{const el=[...document.querySelectorAll("button")].find(b=>(b.textContent||"").trim().toLowerCase()===lbl.toLowerCase());el&&el.click();},label);
  console.log("EN :", await rel());
  await p.screenshot({path:"/out/rel-en1.png"});
  await clickLang("ru"); await s(900);            // toggle to RU — NO reload
  console.log("RU :", await rel());
  await p.screenshot({path:"/out/rel-ru.png"});
  await clickLang("en"); await s(900);            // toggle back to EN — NO reload
  console.log("EN2:", await rel());
  await p.screenshot({path:"/out/rel-en2.png"});
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
