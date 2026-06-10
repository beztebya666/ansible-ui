const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const s=ms=>new Promise(r=>setTimeout(r,ms));
  // shot 1: admin settings → Enable 2FA → setup state (pending, NOT enabled — safe)
  const p=await b.newPage(); await p.setViewport({width:1100,height:900,deviceScaleFactor:2});
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/settings`,{waitUntil:"networkidle2"}); await s(1000);
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Включить 2FA/.test(x.textContent));if(b)b.click();});
  await s(900);
  const clip=await p.evaluate(()=>{const h=[...document.querySelectorAll("h2")].find(x=>/Двухфакторная/.test(x.textContent));if(!h)return null;const c=h.closest(".card");c.scrollIntoView({block:"center"});const r=c.getBoundingClientRect();return {x:Math.max(0,r.left-8),y:Math.max(0,r.top-8),width:r.width+16,height:r.height+16};});
  await s(300);
  await p.screenshot({path:"/out/2fa-setup.png",clip:clip||{x:250,y:200,width:800,height:300}}); console.log("setup shot");
  // shot 2: login page 2FA code step (logged out, login as tfa-user)
  const p2=await b.newPage(); await p2.setViewport({width:900,height:760,deviceScaleFactor:2});
  const ctx=p2.browserContext();
  await p2.goto(base,{waitUntil:"networkidle2"}); await s(500);
  await p2.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p2.goto(base,{waitUntil:"networkidle2"}); await s(600);
  // ensure on login form
  const u2=await p2.$('input[autocomplete="username"]');
  if(u2){await u2.type("tfa-user");await p2.type('input[type="password"]',"tfapass123");await p2.click("button.btn-primary");await s(1200);}
  await p2.screenshot({path:"/out/2fa-login.png"}); console.log("login shot url="+p2.url());
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
