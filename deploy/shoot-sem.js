const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:3000";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1300,height:900}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(1500);
  const inputs=await p.$$('input');
  console.log("inputs on login:",inputs.length);
  // type into the text + password inputs
  for(const el of inputs){const t=await (await el.getProperty('type')).jsonValue(); if(t==='text'||t==='email'){await el.click();await el.type('admin');}}
  for(const el of inputs){const t=await (await el.getProperty('type')).jsonValue(); if(t==='password'){await el.click();await el.type('changeme');}}
  await s(300); await p.keyboard.press('Enter'); await s(3000);
  console.log("after login url="+p.url());
  await p.goto(`${base}/project/1/views/1/templates`,{waitUntil:"networkidle2"}); await s(3000);
  await p.screenshot({path:"/out/sem-templates.png"}); console.log("shot url="+p.url());
  await b.close();
})().catch(e=>{console.error("ERR",e.message);process.exit(1)});
