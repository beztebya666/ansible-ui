const puppeteer=require("puppeteer");
(async()=>{
  const base=process.env.BASE;
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  const clickByText=async(txt)=>p.evaluate((t)=>{const el=[...document.querySelectorAll("button")].find(b=>b.textContent.trim().includes(t));if(el){el.click();return true}return false},txt);
  await p.goto(base,{waitUntil:"networkidle2"}); await s(700);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate(()=>localStorage.setItem("aui.lang","ru"));
  await p.goto(`${base}/environments`,{waitUntil:"networkidle2"}); await s(1200);
  // 1) open Secret backends manager
  await clickByText("Хранилища секретов"); await s(1300);
  await p.screenshot({path:"/out/vault-backends.png"}); console.log("shot backends");
  // close modal (Закрыть) then open env edit
  await clickByText("Закрыть"); await s(600);
  // open edit on vault-env2
  const opened=await p.evaluate(()=>{const rows=[...document.querySelectorAll(".card .flex.items-center")];for(const r of rows){if(r.textContent.includes("vault-env2")){const btn=r.querySelector("button");if(btn){btn.click();return true}}}return false});
  await s(1200);
  // scroll modal to bottom to show external secrets
  await p.evaluate(()=>{const m=document.querySelector('[class*="overflow"]');if(m)m.scrollTop=m.scrollHeight;window.scrollTo(0,document.body.scrollHeight);});
  await s(500);
  await p.screenshot({path:"/out/vault-envform.png"}); console.log("shot envform opened="+opened);
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
