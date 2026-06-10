const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_2e321b4c60baed83";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:1100,deviceScaleFactor:2}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1500);
  const clip=await p.evaluate(()=>{const h=[...document.querySelectorAll("*")].find(e=>(e.textContent||"").trim()==="Команда"||(e.textContent||"").trim()==="COMMAND");if(!h)return null;let c=h;for(let i=0;i<3&&c.parentElement;i++)c=c.parentElement;c.scrollIntoView({block:"center"});const r=c.getBoundingClientRect();return {x:Math.max(0,r.left-10),y:Math.max(0,r.top-10),width:r.width+20,height:r.height+20};});
  await s(400);
  await p.screenshot({path:"/out/cmd-fixed.png",clip:clip||{x:980,y:300,width:320,height:180}}); console.log("clip="+JSON.stringify(clip));
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
