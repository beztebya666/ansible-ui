const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_1dcf9d3552be2e1f";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:1500}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1600);
  await p.evaluate(()=>{const t=[...document.querySelectorAll("*")].filter(e=>(e.textContent||"").trim()==="Артефакты");if(t.length)t[t.length-1].scrollIntoView({block:"center"});});
  await s(500);
  const clip=await p.evaluate(()=>{
    const t=[...document.querySelectorAll("*")].filter(e=>(e.textContent||"").trim()==="Артефакты");
    if(!t.length)return null; let sec=t[t.length-1];
    for(let i=0;i<4&&sec.parentElement;i++){if(sec.getBoundingClientRect().height>70)break;sec=sec.parentElement;}
    const r=sec.getBoundingClientRect();
    return {x:Math.max(0,r.left-14),y:Math.max(0,r.top-14),width:r.width+28,height:r.height+28};
  });
  await p.screenshot({path:"/out/artifacts3.png",clip:clip||{x:980,y:300,width:330,height:200}});
  console.log("clip="+JSON.stringify(clip));
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
