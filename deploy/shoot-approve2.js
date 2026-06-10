const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_0f374d8440ec7660";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1400,height:760}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1500);
  // clip the StatusBadge + buttons region (top-right of page header)
  const clip=await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Одобрить/.test(x.textContent));if(!b)return null;const row=b.parentElement.getBoundingClientRect();return {x:Math.max(0,row.left-12),y:Math.max(0,row.top-10),width:Math.min(620,row.width+24),height:row.height+20};});
  await p.screenshot({path:"/out/approve2.png",clip:clip||{x:700,y:0,width:700,height:70}}); console.log("clip="+JSON.stringify(clip));
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
