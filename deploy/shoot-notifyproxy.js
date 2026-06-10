const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:1000}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  const shoot=async(lang,re,file)=>{
    await p.evaluate(l=>localStorage.setItem("aui.lang",l),lang);
    await p.goto(`${base}/settings`,{waitUntil:"networkidle2"}); await s(1400);
    await p.evaluate(rs=>{const h=[...document.querySelectorAll("h2")].find(e=>new RegExp(rs).test(e.textContent||""));if(h)h.scrollIntoView({block:"center"});},re);
    await s(600);
    await p.screenshot({path:`/out/${file}`}); console.log("shot",file);
  };
  await shoot("ru","Прокси для отправки","notifyproxy-ru.png");
  await shoot("en","Notification egress","notifyproxy-en.png");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
