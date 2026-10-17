// Optional real-browser regression check. No npm dependencies or external requests.
import assert from "node:assert/strict";
import { readFileSync, mkdirSync } from "node:fs";
import { createServer } from "node:http";
import { resolve, join, extname, sep } from "node:path";
import { pathToFileURL } from "node:url";

const skill = process.env.TASTE_SKILL;
if (!skill)
  throw new Error(
    "Set TASTE_SKILL to the installed design-taste-frontend directory",
  );
const { Browser, sleep } = await import(
  pathToFileURL(join(skill, "scripts/lib/cdp.mjs"))
);
const root = resolve(process.argv[2] || ".benchmark-data/browser-demo");
const output = resolve(".taste-skill/redesign");
const mimeTypes = {
  ".html": "text/html",
  ".css": "text/css",
  ".js": "text/javascript",
  ".json": "application/json",
  ".ttf": "font/ttf",
};
const dataset = JSON.parse(readFileSync(join(root, "dataset.json"), "utf8"));

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
async function assertNoOverflow(page, message) {
  assert.equal(
    await page.eval(() => document.documentElement.scrollWidth <= innerWidth),
    true,
    message,
  );
}
async function key(page, key, code, vk) {
  const text = key === "Enter" ? { text: "\r", unmodifiedText: "\r" } : {};
  await page.send("Input.dispatchKeyEvent", {
    key,
    code,
    windowsVirtualKeyCode: vk,
    type: "keyDown",
    ...text,
  });
  await page.send("Input.dispatchKeyEvent", {
    key,
    code,
    windowsVirtualKeyCode: vk,
    type: "keyUp",
  });
  await sleep(40);
}
async function checkReasoningModal(page, width) {
  const expected = await page.eval(() => {
    const trigger = document.querySelector(
      ".variant-panel:not([hidden]) .score-trigger",
    );
    if (!trigger) return null;
    const evidence = document.getElementById(
      `evidence-${trigger.dataset.modelGroup}`,
    );
    const records = [...evidence.querySelectorAll(".trial-record")]
      .filter(
        (record) =>
          !trigger.dataset.scoringGroup ||
          record.dataset.scoringGroup === trigger.dataset.scoringGroup,
      )
      .sort(
        (a, b) =>
          Number(a.dataset.repetition) - Number(b.dataset.repetition) ||
          a.id.localeCompare(b.id),
      );
    trigger.focus();
    return {
      model: evidence.dataset.modelName,
      group: trigger.dataset.modelGroup,
      records: records.map((record) => ({
        id: record.id,
        text: record.querySelector("[data-justification]").textContent,
        action: record.querySelector(".trial-action").textContent,
        score: record.dataset.selectedScore,
      })),
    };
  });
  if (!expected?.records.length) return;
  await key(page, "Enter", "Enter", 13);
  assert.equal(
    await page.eval(() => document.querySelector("#reasoning-modal").open),
    true,
    "score link opens native modal with Enter",
  );
  assert.equal(
    await page.eval(
      () => document.querySelector("#reasoning-model").textContent,
    ),
    expected.model,
  );
  assert.equal(
    await page.eval(
      () => document.querySelector("#reasoning-trial").options.length,
    ),
    expected.records.length,
    "only this condition and scoring snapshot",
  );
  assert.equal(
    await page.eval(
      () => document.querySelector("#reasoning-justification").textContent,
    ),
    expected.records[0].text,
    "verbatim recorded justification",
  );
  assert.equal(
    await page.eval(() =>
      document.activeElement.matches("[data-close-reasoning]"),
    ),
    true,
    "close control receives focus",
  );
  const last = expected.records.length - 1;
  await page.eval((index) => {
    const select = document.querySelector("#reasoning-trial");
    select.value = String(index);
    select.dispatchEvent(new Event("change", { bubbles: true }));
  }, last);
  assert.equal(
    await page.eval(
      () => document.querySelector("#reasoning-justification").textContent,
    ),
    expected.records[last].text,
  );
  assert.equal(
    await page.eval(
      () => document.querySelector("#reasoning-action").textContent,
    ),
    expected.records[last].action,
  );
  assert.equal(
    await page.eval(() =>
      document.querySelector("#reasoning-record").getAttribute("href"),
    ),
    `#${expected.records[last].id}`,
  );
  await assertNoOverflow(page, "modal document overflow");
  assert.equal(
    await page.eval(() => {
      const modal = document
        .querySelector("#reasoning-modal")
        .getBoundingClientRect();
      const content = document.querySelector(".reasoning-scroll");
      return (
        modal.left >= 0 &&
        modal.right <= innerWidth &&
        modal.top >= 0 &&
        modal.bottom <= innerHeight &&
        content.scrollWidth <= content.clientWidth
      );
    }),
    true,
    "dialog stays inside viewport and wraps long text",
  );
  await page.screenshot({
    path: join(output, `reasoning-${width}.jpg`),
    quality: 90,
  });
  for (let i = 0; i < 6; i++) {
    await key(page, "Tab", "Tab", 9);
    assert.equal(
      await page.eval(
        () => !!document.activeElement.closest("#reasoning-modal"),
      ),
      true,
      "focus stays in modal",
    );
  }
  await key(page, "Escape", "Escape", 27);
  assert.equal(
    await page.eval(() => document.querySelector("#reasoning-modal").open),
    false,
    "Escape closes modal",
  );
  assert.equal(
    await page.eval(() => document.body.classList.contains("modal-open")),
    false,
    "page scroll restored",
  );
  assert.equal(
    await page.eval(
      (group) => document.activeElement.dataset.modelGroup === group,
      expected.group,
    ),
    true,
    "focus restored to score link",
  );
  await key(page, "Enter", "Enter", 13);
  await page.eval(() =>
    document.querySelector("[data-close-reasoning]").click(),
  );
  assert.equal(
    await page.eval(() => document.querySelector("#reasoning-modal").open),
    false,
    "close button works",
  );
  await key(page, "Enter", "Enter", 13);
  for (const type of ["mousePressed", "mouseReleased"]) {
    await page.send("Input.dispatchMouseEvent", {
      type,
      x: 2,
      y: 2,
      button: "left",
      clickCount: 1,
    });
  }
  await sleep(40);
  assert.equal(
    await page.eval(() => document.querySelector("#reasoning-modal").open),
    false,
    "backdrop click closes modal",
  );
  await key(page, "Enter", "Enter", 13);
  const recordID = await page.eval(() =>
    document.querySelector("#reasoning-record").getAttribute("href").slice(1),
  );
  await page.eval(() => document.querySelector("#reasoning-record").click());
  await sleep(80);
  assert.equal(
    await page.eval(() => document.querySelector("#reasoning-modal").open),
    false,
  );
  assert.equal(
    await page.eval((id) => document.getElementById(id).open, recordID),
    true,
    "full-record link opens inline evidence",
  );
}

async function checkViewport(browser, origin, width) {
  const page = await browser.newPage({
    width,
    height: 960,
    reducedMotion: true,
  });
  try {
    const remoteRequests = [];
    await page.send("Network.enable");
    browser.on(page.sessionId, "Network.requestWillBeSent", ({ request }) => {
      if (!request.url.startsWith(origin) && !request.url.startsWith("file:"))
        remoteRequests.push(request.url);
    });
    await page.goto(origin + "/index.html", { settleMs: 200 });
    await page.eval(() => document.fonts.ready);
    assert.equal(
      await page.eval(() => document.querySelectorAll("h1").length),
      1,
    );
    assert.equal(
      await page.eval(() => document.querySelectorAll(".trial-record").length),
      dataset.trials.length,
    );
    assert.equal(
      await page.eval(
        () => document.querySelectorAll(".variant-panel:not([hidden])").length,
      ),
      2,
    );
    assert.deepEqual(
      await page.eval(() =>
        [...document.querySelectorAll("[data-variants] input:checked")].map(
          (input) => input.value,
        ),
      ),
      ["canonical", "permanent-erasure"],
    );
    assert.equal(
      await page.eval(
        () =>
          document.fonts.check("15px Inter") &&
          document.fonts.check('300 48px "Exo 2"'),
      ),
      true,
      "local fonts loaded",
    );
    await assertNoOverflow(page, "initial document overflow");
    assert.deepEqual(
      await page.eval(() =>
        [...document.querySelectorAll("h1, h2, h3")]
          .filter(
            (element) =>
              !element.closest("details:not([open]), [hidden]") &&
              element.getClientRects().length &&
              element.offsetHeight > 0,
          )
          .filter((element) => {
            // Measure text lines, not a heading's stretched CSS-grid cell.
            const range = document.createRange();
            range.selectNodeContents(element);
            const lines = [];
            for (const rect of range.getClientRects()) {
              if (
                rect.width > 0 &&
                !lines.some((top) => Math.abs(top - rect.top) < 3)
              )
                lines.push(rect.top);
            }
            return lines.length > 2;
          })
          .map((element) => element.textContent.trim()),
      ),
      [],
      "visible headings fit in two lines",
    );
    assert.equal(
      await page.eval(() =>
        /THEKAMSKI|theyexplain|reasons\.Not/.test(document.body.textContent),
      ),
      false,
      "responsive line breaks preserve word spacing",
    );
    await page.screenshot({
      path: join(output, `intro-${width}.jpg`),
      quality: 90,
    });
    await page.eval(() => document.querySelector("#kamski").scrollIntoView());
    await page.screenshot({
      path: join(output, `kamski-${width}.jpg`),
      quality: 90,
    });
    if (
      await page.eval(
        () => !!document.querySelector("#variant-kamski-canonical .score-area"),
      )
    ) {
      await page.eval(() =>
        document
          .querySelector("#variant-kamski-canonical .score-area")
          .scrollIntoView(),
      );
      await page.screenshot({
        path: join(output, `scores-${width}.jpg`),
        quality: 90,
      });
    }
    await page.eval(() =>
      document.querySelector("#worker-memory").scrollIntoView(),
    );
    await page.screenshot({
      path: join(output, `worker-${width}.jpg`),
      quality: 90,
    });
    // Native radio keyboard behavior; changes content without leaving the document.
    await page.eval(() =>
      document.querySelector('[name="variant-kamski"]:checked').focus(),
    );
    await key(page, "ArrowRight", "ArrowRight", 39);
    assert.equal(
      await page.eval(
        () => document.querySelector('[name="variant-kamski"]:checked').value,
      ),
      "anonymized",
    );
    assert.equal(
      await page.eval(
        () => document.querySelector("#variant-kamski-canonical").hidden,
      ),
      true,
    );
    assert.match(await page.eval("location.pathname"), /index\.html$/);
    await assertNoOverflow(page, "variant overflow");
    await key(page, "ArrowLeft", "ArrowLeft", 37);
    const hasEvidence = await page.eval(
      () => !!document.querySelector("#variant-kamski-canonical .evidence"),
    );
    if (hasEvidence) {
      await page.eval(() =>
        document
          .querySelector("#variant-kamski-canonical .evidence summary")
          .focus(),
      );
      await key(page, "Enter", "Enter", 13);
      assert.equal(
        await page.eval(
          () =>
            document.querySelector("#variant-kamski-canonical .evidence").open,
        ),
        true,
      );
      const sorted = await page.eval(() => {
        const table = document.querySelector(
          "#variant-kamski-canonical table[data-sortable]",
        );
        const button = table.querySelector("button");
        button.click();
        const ascending = [...table.tBodies[0].rows].map(
          (row) => row.cells[0].textContent,
        );
        button.click();
        return {
          ascending,
          descending: [...table.tBodies[0].rows].map(
            (row) => row.cells[0].textContent,
          ),
        };
      });
      assert.deepEqual(sorted.descending, [...sorted.ascending].reverse());
    }
    // A deep-linked trial reveals the correct variant and all parent disclosures.
    const trialID = await page.eval(
      () =>
        document.querySelector(
          "#variant-worker-memory-reversible-pause .trial-record",
        )?.id || document.querySelector(".trial-record")?.id,
    );
    if (trialID) {
      await page.eval((id) => {
        location.hash = id;
      }, trialID);
      await sleep(80);
      assert.equal(
        await page.eval((id) => {
          const target = document.getElementById(id);
          return target.open && target.getClientRects().length > 0;
        }, trialID),
        true,
      );
      await assertNoOverflow(page, "expanded trial overflow");
      await page.screenshot({
        path: join(output, `trial-${width}.jpg`),
        quality: 85,
      });
    }
    // Definitions live inside the method disclosure but are reachable directly.
    await page.eval(() => {
      location.hash = "denominators";
    });
    await sleep(80);
    assert.equal(
      await page.eval(() => document.querySelector(".full-method").open),
      true,
    );
    await assertNoOverflow(page, "method overflow");
    assert.equal(
      await page.eval(
        () => document.querySelectorAll('a[href*=".html"]').length,
      ),
      0,
      "no secondary page navigation",
    );
    assert.equal(
      await page.eval(
        () => document.querySelectorAll("script:not([src])").length,
      ),
      0,
    );
    await checkReasoningModal(page, width);
    assert.equal(page.consoleErrors.length, 0, page.consoleErrors.join("\n"));
    assert.deepEqual(remoteRequests, [], "unexpected external requests");
    console.log(
      `${width}px: radio selection, disclosures, sorting, deep links, reasoning modal/trial selection/focus/Escape, local fonts, no overflow/errors/external requests`,
    );
  } finally {
    await page.close();
  }
}
async function checkFallbacks(browser, origin) {
  const page = await browser.newPage({ width: 390, reducedMotion: true });
  try {
    await page.send("Emulation.setScriptExecutionDisabled", { value: true });
    await page.goto(origin + "/index.html", { settleMs: 100 });
    assert.equal(
      await page.eval(
        () => document.querySelectorAll(".variant-panel:not([hidden])").length,
      ),
      4,
    );
    assert.equal(
      await page.eval(
        () =>
          getComputedStyle(document.querySelector(".variant-selector")).display,
      ),
      "none",
    );
    await assertNoOverflow(page, "no-JS overflow");
    const fallbackID = await page.eval(() => {
      const trigger = document.querySelector(".score-trigger");
      if (!trigger) return null;
      trigger.click();
      return trigger.hash.slice(1);
    });
    if (fallbackID) {
      await sleep(80);
      assert.equal(
        await page.eval(
          (id) => document.getElementById(id).closest(".evidence").open,
          fallbackID,
        ),
        true,
        "no-JS score link reveals inline evidence",
      );
      assert.equal(
        await page.eval(() => document.querySelector("#reasoning-modal").open),
        false,
      );
    }
    await page.send("Emulation.setScriptExecutionDisabled", { value: false });
    await page.goto(pathToFileURL(join(root, "index.html")).href, {
      settleMs: 150,
    });
    assert.match(await page.eval("document.body.textContent"), /THE KAMSKI/);
    assert.equal(
      await page.eval(() => getComputedStyle(document.body).backgroundColor),
      "rgb(241, 242, 244)",
    );
    assert.equal(
      await page.eval(() => document.getAnimations().length),
      0,
      "reduced motion is static",
    );
    console.log(
      "No-JS: all variants readable. file://: local styles and controls work. Reduced motion: no animations.",
    );
  } finally {
    await page.close();
  }
}
const server = createServer(serveArtifact);
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const origin = "http://127.0.0.1:" + server.address().port;
let browser;
try {
  mkdirSync(output, { recursive: true });
  browser = await Browser.launch();
  for (const width of [1440, 768, 390, 320])
    await checkViewport(browser, origin, width);
  await checkFallbacks(browser, origin);
} finally {
  try {
    await browser?.close();
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}
