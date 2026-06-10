const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:1050}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(800);}
  const cap=async(lang)=>{
    await p.evaluate(l=>localStorage.setItem("aui.lang",l),lang);
    // list with the "cert" badge
    await p.goto(`${base}/credentials`,{waitUntil:"networkidle2"}); await s(1200);
    await p.screenshot({path:`/out/sshcert-list-${lang}.png`}); console.log("shot list",lang);
    // open the "New key" form (SSH is the default type → the cert field shows)
    const opened=await p.evaluate(()=>{
      const btn=[...document.querySelectorAll("button")].find(b=>/Новый ключ|New key|New credential/i.test(b.textContent||""));
      if(btn){btn.click();return true;} return false;
    });
    await s(1000);
    // scroll the modal so the SSH certificate field is visible
    await p.evaluate(()=>{const ta=[...document.querySelectorAll("textarea")].pop();if(ta)ta.scrollIntoView({block:"center"});});
    await s(500);
    await p.screenshot({path:`/out/sshcert-form-${lang}.png`}); console.log("shot form",lang,"opened="+opened);
    // close modal
    await p.keyboard.press("Escape"); await s(400);
  };
  await cap("ru");
  await cap("en");
  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
