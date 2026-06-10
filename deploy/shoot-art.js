const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080",rid="run_1dcf9d3552be2e1f";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/runs/${rid}`,{waitUntil:"networkidle2"}); await s(1600);
  // scroll the sidebar to the Artifacts section
  await p.evaluate(()=>{const el=[...document.querySelectorAll("*")].find(e=>/Артефакты/.test((e.textContent||"").trim())&&e.children.length<3);if(el)el.scrollIntoView({block:"center"});});
  await s(500);
  const aside=await p.$("aside");
  if(aside){const box=await aside.boundingBox(); await p.screenshot({path:"/out/artifacts.png",clip:{x:Math.max(0,box.x-4),y:60,width:box.width+8,height:760}});}
  else await p.screenshot({path:"/out/artifacts.png"});
  console.log("shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
