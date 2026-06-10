const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_1dcf9d3552be2e1f";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1600);
  const clip=await p.evaluate(()=>{
    // the Section whose title is "Артефакты"
    const titles=[...document.querySelectorAll("*")].filter(e=>(e.textContent||"").trim()==="Артефакты");
    if(!titles.length)return null;
    let sec=titles[titles.length-1];
    // climb to the surrounding card/section block
    for(let i=0;i<4&&sec.parentElement;i++){if(sec.getBoundingClientRect().height>60)break;sec=sec.parentElement;}
    const r=sec.getBoundingClientRect();
    return {x:Math.max(0,r.left-12),y:Math.max(0,r.top-12),width:r.width+24,height:r.height+24};
  });
  await p.screenshot({path:"/out/artifacts2.png",clip:clip||{x:980,y:60,width:320,height:300}});
  console.log("clip="+JSON.stringify(clip));
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
