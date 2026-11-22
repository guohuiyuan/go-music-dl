const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const source = fs.readFileSync(path.join(__dirname, "../templates/static/js/app.js"), "utf8");
const start = source.indexOf("async function loadDownloadQueue(");
const end = source.indexOf("async function downloadQueueAction(", start);
assert.ok(start >= 0 && end > start, "locate the production queue renderer");
const escapeStart = source.indexOf("function escapeHtml(");
const escapeEnd = source.indexOf("\n}\n", escapeStart) + 3;
assert.ok(escapeStart >= 0 && escapeEnd > escapeStart, "locate the production HTML escaper");

// Execute the production renderer with only its network and DOM boundaries stubbed.
async function render(tasks) {
  const elements = new Map();
  const context = vm.createContext({
    URLSearchParams,
    API_ROOT: "/music",
    downloadQueuePage: 1,
    downloadQueueLoading: false,
    downloadQueueFilter: "",
    downloadQueueRenderKey: "",
    downloadQueueLastPoll: 0,
    document: { getElementById(id) {
      if (!elements.has(id)) elements.set(id, { innerHTML: "", textContent: "" });
      return elements.get(id);
    } },
    fetch: async () => ({ ok: true, json: async () => ({
      page: 1, total_pages: 1, counts: { success: tasks.length }, tasks,
    }) }),
    setDownloadRecordsButtonState() {},
    renderUtilityModalPagination: () => "",
  });
  vm.runInContext(source.slice(escapeStart, escapeEnd), context);
  vm.runInContext(source.slice(start, end), context);
  await vm.runInContext("loadDownloadQueue()", context);
  assert.equal(elements.get("download-queue-error").textContent, "");
  return elements.get("download-queue-list").innerHTML;
}

const task = { ID: 1, Name: "Test", Artist: "Artist", Source: "test", Status: "success", Path: "/music/test.mp3" };

test("WebDAV failure is shown as a warning while the local file stays accessible", async () => {
  const html = await render([{ ...task, Warning: " WebDAV: upload failed: 503" }]);
  assert.match(html, /queue-status-warning/);
  assert.match(html, /下载完成，有警告/);
  assert.match(html, /WebDAV: upload failed: 503/);
  assert.match(html, /data-reveal-download-queue="1"/);
  assert.doesNotMatch(html, /queue-status-success/);
});

test("metadata warnings also prevent an unqualified success label", async () => {
  const html = await render([{ ...task, Warning: "歌词写入失败" }]);
  assert.match(html, /queue-status-warning/);
  assert.match(html, /歌词写入失败/);
});

test("successful downloads without warnings keep the success label", async () => {
  const html = await render([task]);
  assert.match(html, /queue-status-success/);
  assert.match(html, /下载成功/);
  assert.match(html, /\/music\/test.mp3/);
  assert.doesNotMatch(html, /queue-status-warning/);
});

test("warnings do not hide a failed task's error or retry action", async () => {
  const html = await render([{ ...task, Status: "failed", Error: "download failed", Warning: "WebDAV failure" }]);
  assert.match(html, /queue-status-failed/);
  assert.match(html, /下载失败/);
  assert.match(html, /download failed/);
  assert.match(html, /downloadQueueAction\(1,'retry'\)/);
  assert.doesNotMatch(html, /queue-status-warning/);
});

test("warning details are escaped before rendering", async () => {
  const html = await render([{ ...task, Warning: '<img src=x onerror="bad()">' }]);
  assert.match(html, /&lt;img/);
  assert.doesNotMatch(html, /<img/);
});

test("blank warnings do not change success status", async () => {
  const html = await render([{ ...task, Warning: " \n\t " }]);
  assert.match(html, /queue-status-success/);
  assert.doesNotMatch(html, /queue-status-warning/);
});

test("a mixed batch marks only the affected download with a warning", async () => {
  const html = await render([task, { ...task, ID: 2, Warning: "WebDAV: upload failed" }]);
  assert.equal((html.match(/queue-status-success/g) || []).length, 1);
  assert.equal((html.match(/queue-status-warning/g) || []).length, 1);
});

for (const [status, label] of [["waiting", "等待下载"], ["skipped", "已下载，跳过"]]) {
  test(`${status} tasks retain their original label`, async () => {
    const html = await render([{ ...task, Status: status, Warning: "old warning" }]);
    assert.match(html, new RegExp(`queue-status-${status}`));
    assert.ok(html.includes(label));
    assert.doesNotMatch(html, /queue-status-warning/);
  });
}
