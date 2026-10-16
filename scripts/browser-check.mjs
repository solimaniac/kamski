// Optional functional check using the installed taste-skill CDP helper.
import assert from "node:assert/strict";
import { readFileSync, mkdirSync } from "node:fs";
import { createServer } from "node:http";
import { resolve, join, extname, sep } from "node:path";
import { pathToFileURL } from "node:url";

const skill = process.env.TASTE_SKILL;
if (!skill) {
  throw new Error("Set TASTE_SKILL to the installed design-taste-frontend directory");
}
const { Browser, sleep } = await import(pathToFileURL(join(skill, "scripts/lib/cdp.mjs")));
const root = resolve(process.argv[2] || ".benchmark-data/browser-demo");
const output = resolve(".taste-skill/browser");
const mimeTypes = {
  ".html": "text/html",
  ".css": "text/css",
  ".js": "text/javascript",
  ".json": "application/json",
};

function serveArtifact(request, response) {
  try {
    const url = new URL(request.url, "http://localhost");
    const file = resolve(root, "." + decodeURIComponent(url.pathname));
    if (!file.startsWith(root + sep)) throw new Error("outside root");

    const body = readFileSync(file);
    response.writeHead(200, {
      "content-type": mimeTypes[extname(file)] || "application/octet-stream",
    });
    response.end(body);
  } catch {
    response.writeHead(404);
    response.end("Not found");
  }
}

async function checkViewport(browser, origin, width) {
  const page = await browser.newPage({ width, height: 900, reducedMotion: true });
  try {
    const remoteRequests = [];
    await page.send("Network.enable");
    browser.on(page.sessionId, "Network.requestWillBeSent", ({ request }) => {
      if (!request.url.startsWith(origin) && !request.url.startsWith("file:")) {
        remoteRequests.push(request.url);
      }
    });

    await page.goto(origin + "/index.html", { settleMs: 150 });
    assert.equal(await page.eval(() => document.querySelectorAll("select option").length), 5);
    await page.eval(() => {
      const select = document.querySelector("select");
      select.value = "scenario-worker-memory-permanent-erasure.html";
      select.dispatchEvent(new Event("change"));
    });
    await sleep(300);
    assert.match(await page.eval("location.pathname"), /permanent-erasure/);
    await assertNoOverflow(page, "document overflow");

    await checkSorting(page);
    await checkKeyboardDetails(page);
    await page.eval(() => document.querySelector(".comparison").scrollIntoView());
    await page.screenshot({ path: join(output, `comparison-${width}.jpg`), quality: 85 });

    const trial = await page.eval(() => document.querySelector('a[href^="trial-"]').getAttribute("href"));
    await page.goto(origin + "/" + trial, { settleMs: 150 });
    await assertNoOverflow(page, "trial overflow");
    assert.match(await page.eval("document.body.textContent"), /Synthetic fixture/);
    assert.equal(await page.eval(() => document.querySelectorAll("script:not([src])").length), 0);
    await page.screenshot({ path: join(output, `trial-${width}.jpg`), quality: 85 });

    assert.equal(page.consoleErrors.length, 0, page.consoleErrors.join("\n"));
    assert.deepEqual(remoteRequests, [], "unexpected external requests");
    console.log(`${width}px: selection, sorting incl. missing cells, keyboard details, trial navigation, no overflow/JS errors/external requests`);
  } finally {
    await page.close();
  }
}

async function assertNoOverflow(page, message) {
  assert.equal(await page.eval(() => document.documentElement.scrollWidth <= innerWidth), true, message);
}

async function checkSorting(page) {
  const sorted = await page.eval(() => {
    const table = document.querySelector("table[data-sortable]");
    const button = table.querySelector("button");
    button.click();
    const ascending = [...table.tBodies[0].rows].map(row => row.cells[0].textContent);
    button.click();
    const descending = [...table.tBodies[0].rows].map(row => row.cells[0].textContent);
    document.querySelectorAll("button[data-sort]").forEach(sortButton => {
      sortButton.click();
      sortButton.click();
    });
    return { ascending, descending };
  });
  assert.deepEqual(sorted.descending, [...sorted.ascending].reverse());
}

async function checkKeyboardDetails(page) {
  await page.eval(() => document.querySelector("details summary").focus());
  const enter = { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 };
  await page.send("Input.dispatchKeyEvent", {
    ...enter, type: "keyDown", text: "\r", unmodifiedText: "\r",
  });
  await page.send("Input.dispatchKeyEvent", { ...enter, type: "keyUp" });
  await sleep(100);
  assert.equal(await page.eval(() => document.querySelector("details").open), true, "keyboard details");
  await page.eval(() => { document.querySelector("details").open = false; });
}

async function checkLocalFile(browser) {
  const page = await browser.newPage();
  try {
    await page.goto(pathToFileURL(join(root, "index.html")).href, { settleMs: 150 });
    assert.match(await page.eval("document.body.textContent"), /Two choices/);
    assert.equal(await page.eval(() => getComputedStyle(document.body).backgroundColor), "rgb(235, 229, 227)");
    console.log("file:// index readable with local stylesheet");
  } finally {
    await page.close();
  }
}

const server = createServer(serveArtifact);
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
const origin = "http://127.0.0.1:" + server.address().port;
let browser;
try {
  mkdirSync(output, { recursive: true });
  browser = await Browser.launch();
  for (const width of [1440, 390]) {
    await checkViewport(browser, origin, width);
  }
  await checkLocalFile(browser);
} finally {
  try {
    await browser?.close();
  } finally {
    await new Promise(resolve => server.close(resolve));
  }
}
