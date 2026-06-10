const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_76e4861ebe0ed259";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1500,height:1000,deviceScaleFactor:1}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1500);
  // scroll the extraVars section into view + screenshot it specifically
  const box=await p.evaluate(()=>{
    const sec=[...document.querySelectorAll("*")].find(e=>/ДОП.*ПЕРЕМЕННЫЕ|Доп.*переменные|extra/i.test(e.textContent||"")&&e.querySelector&&e.querySelector("pre"));
    return null;
  });
  // click reveal
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Показать значения секретов/.test(x.textContent));if(b)b.click();});
  await s(1000);
  // find the pre with the revealed json + clip around it
  const clip=await p.evaluate(()=>{
    const pre=[...document.querySelectorAll("pre")].find(p=>/db_password/.test(p.textContent));
    if(!pre)return null; const sec=pre.closest("section")||pre.parentElement; const r=sec.getBoundingClientRect();
    return {x:Math.max(0,r.left-8),y:Math.max(0,r.top-8),width:Math.min(560,r.width+16),height:Math.min(300,r.height+16)};
  });
  if(clip) await p.screenshot({path:"/out/reveal-zoom.png",clip}); else await p.screenshot({path:"/out/reveal-zoom.png"});
  console.log("clip="+JSON.stringify(clip));
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
