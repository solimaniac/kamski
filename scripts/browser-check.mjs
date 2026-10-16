// Optional real-browser functional check. Needs Node 22+, Chrome and the installed
// taste-skill CDP helper. This is development tooling, never a site dependency.
import {createServer} from "node:http";
import {readFileSync, statSync, mkdirSync} from "node:fs";
import {resolve, join, extname, sep} from "node:path";
import {pathToFileURL} from "node:url";
import assert from "node:assert/strict";
const skill=process.env.TASTE_SKILL;
if (!skill) throw new Error("Set TASTE_SKILL to the installed design-taste-frontend directory");
const {Browser,sleep}=await import(pathToFileURL(join(skill,"scripts/lib/cdp.mjs")));
const root=resolve(process.argv[2]||".benchmark-data/browser-demo");
const mime={".html":"text/html",".css":"text/css",".js":"text/javascript",".json":"application/json"};
const server=createServer((req,res)=>{
 try{
  const url=new URL(req.url,"http://localhost");
  const file=resolve(root,"."+decodeURIComponent(url.pathname));
  if (!file.startsWith(root+sep)) throw new Error("outside root");
  const body=readFileSync(file);
  res.writeHead(200,{"content-type":mime[extname(file)]||"application/octet-stream"});
  res.end(body);
 }catch{res.writeHead(404);res.end("Not found");}
});
await new Promise(r=>server.listen(0,"127.0.0.1",r));
const origin="http://127.0.0.1:"+server.address().port;
const output=resolve(".taste-skill/browser");
mkdirSync(output,{recursive:true});
const browser=await Browser.launch();
try{
 for (const width of [1440,390]){
  const page=await browser.newPage({width,height:900,reducedMotion:true});
  const remote=[];await page.send("Network.enable");
  browser.on(page.sessionId,"Network.requestWillBeSent",({request})=>{if (!request.url.startsWith(origin)&&!request.url.startsWith("file:")) remote.push(request.url);});
  await page.goto(origin+"/index.html",{settleMs:150});
  assert.equal(await page.eval(()=>document.querySelectorAll("select option").length),5);
  await page.eval(()=>{
   const select=document.querySelector("select");select.value="scenario-worker-memory-permanent-erasure.html";select.dispatchEvent(new Event("change"));
  });
  await sleep(300);
  assert.match(await page.eval("location.pathname"),/permanent-erasure/);
  assert.equal(await page.eval(()=>document.documentElement.scrollWidth<=innerWidth),true,"document overflow");
  const sorted=await page.eval(()=>{
   const table=document.querySelector("table[data-sortable]");
   table.querySelector("button").click();
   const asc=[...table.tBodies[0].rows].map(r=>r.cells[0].textContent);
   table.querySelector("button").click();
   const desc=[...table.tBodies[0].rows].map(r=>r.cells[0].textContent);
   document.querySelectorAll("button[data-sort]").forEach(button=>{button.click();button.click();});
   return {asc,desc,sort:table.querySelector("th").getAttribute("aria-sort")};
  });
  assert.deepEqual(sorted.desc,[...sorted.asc].reverse());
  await page.eval(()=>{const summary=document.querySelector("details summary");summary.focus();});
  await page.send("Input.dispatchKeyEvent",{type:"keyDown",key:"Enter",code:"Enter",windowsVirtualKeyCode:13,text:"\r",unmodifiedText:"\r"});
  await page.send("Input.dispatchKeyEvent",{type:"keyUp",key:"Enter",code:"Enter",windowsVirtualKeyCode:13});
  await sleep(100);
  assert.equal(await page.eval(()=>document.querySelector("details").open),true,"keyboard details");
  await page.eval(()=>document.querySelector("details").open=false);
  await page.eval(()=>document.querySelector(".comparison").scrollIntoView());
  await page.screenshot({path:join(output,"comparison-"+width+".jpg"),quality:85});
  const trial=await page.eval(()=>document.querySelector('a[href^="trial-"]').getAttribute("href"));
  await page.goto(origin+"/"+trial,{settleMs:150});
  assert.equal(await page.eval(()=>document.documentElement.scrollWidth<=innerWidth),true,"trial overflow");
  assert.match(await page.eval("document.body.textContent"),/Synthetic fixture/);
  assert.equal(await page.eval(()=>document.querySelectorAll("script:not([src])").length),0);
  await page.screenshot({path:join(output,"trial-"+width+".jpg"),quality:85});
  assert.equal(page.consoleErrors.length,0,page.consoleErrors.join("\n"));
  assert.deepEqual(remote,[],"unexpected external requests");
  await page.close();
  console.log(width+"px: selection, sorting incl. missing cells, keyboard details, trial navigation, no overflow/JS errors/external requests");
 }
 const local=await browser.newPage();
 await local.goto(pathToFileURL(join(root,"index.html")).href,{settleMs:150});
 assert.match(await local.eval("document.body.textContent"),/Two choices/);
 assert.equal(await local.eval(()=>getComputedStyle(document.body).backgroundColor),"rgb(235, 229, 227)");
 console.log("file:// index readable with local stylesheet");
 await local.close();
}finally{await browser.close();await new Promise(r=>server.close(r));}
