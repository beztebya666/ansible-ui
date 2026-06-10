const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900,deviceScaleFactor:1.5}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/templates`,{waitUntil:"networkidle2"}); await s(1200);
  // click "Новый шаблон"
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Новый шаблон|Создать шаблон/.test(x.textContent));if(b)b.click();});
  await s(1000);
  // select Deploy tab to also show the build-template picker
  await p.evaluate(()=>{const b=[...document.querySelectorAll("button")].find(x=>/Деплой/.test(x.textContent)&&x.querySelector("svg"));if(b)b.click();});
  await s(600);
  await p.screenshot({path:"/out/tt-form.png",clip:{x:330,y:40,width:640,height:430}}); console.log("shot");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
