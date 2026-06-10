const puppeteer=require("puppeteer");
(async()=>{
  const base="http://localhost:8080";
  const RID=process.env.RID||"", PROJ=process.env.PROJ||"";
  const b=await puppeteer.launch({headless:"new",executablePath:process.env.PUPPETEER_EXECUTABLE_PATH||"/usr/bin/chromium-browser",args:["--no-sandbox","--disable-gpu","--disable-dev-shm-usage"]});
  const p=await b.newPage(); await p.setViewport({width:1440,height:1040,deviceScaleFactor:1}); const s=ms=>new Promise(r=>setTimeout(r,ms));
  await p.goto(base,{waitUntil:"networkidle2"}); await s(600);
  if(await p.$('input[type="password"]')){const u=await p.$('input[autocomplete="username"]');if(u)await u.type("admin");await p.type('input[type="password"]',"admin123");await p.click("button.btn-primary");await p.waitForSelector("aside",{timeout:10000}).catch(()=>{});await s(900);}
  await p.evaluate((pr)=>{localStorage.setItem("aui.lang","en");localStorage.setItem("aui.project",pr);},PROJ);

  // 1. Dashboard
  await p.goto(`${base}/`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/dashboard.png"}); console.log("shot dashboard");

  // 2. Live terminal (run detail — colored PTY output)
  await p.goto(`${base}/runs/${RID}`,{waitUntil:"networkidle2"}); await s(2200);
  await p.screenshot({path:"/out/terminal.png"}); console.log("shot terminal");

  // 3. Workflows DAG (page already scopes to the demo project via aui.project)
  await p.goto(`${base}/workflows`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/workflows.png"}); console.log("shot workflows");

  // 4. Cluster / HA
  await p.goto(`${base}/cluster`,{waitUntil:"networkidle2"}); await s(1600);
  await p.screenshot({path:"/out/cluster.png"}); console.log("shot cluster");

  // 5. Insights / analytics
  await p.goto(`${base}/insights`,{waitUntil:"networkidle2"}); await s(1800);
  await p.screenshot({path:"/out/insights.png"}); console.log("shot insights");

  // 6. Runs list
  await p.goto(`${base}/runs`,{waitUntil:"networkidle2"}); await s(1500);
  await p.screenshot({path:"/out/runs.png"}); console.log("shot runs");

  await b.close();
})().catch(e=>{console.error(e);process.exit(1)});
