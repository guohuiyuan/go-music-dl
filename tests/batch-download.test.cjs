const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const app = fs.readFileSync('internal/web/templates/static/js/app.js', 'utf8');
const code = app.slice(app.indexOf('function confirmBatchDownload('), app.indexOf('async function deleteLocalMusic('));

function setup() {
  let dialog;
  let downloads = 0;
  let restored = 0;
  const button = { innerHTML: '批量下载', disabled: false };
  const context = {
    document: {
      activeElement: { isConnected: true, focus() { restored++; } },
      createElement() {
        dialog = {
          returnValue: '', message: {}, listeners: {},
          setAttribute() {}, querySelector() { return this.message; },
          addEventListener(event, handler) { this.listeners[event] = handler; },
          showModal() { this.open = true; }, remove() { this.open = false; },
          close(value = '') { this.returnValue = value; this.listeners.close(); },
        };
        return dialog;
      },
      body: { appendChild() {} },
      getElementById() { return button; }, querySelectorAll() { return [1]; },
    },
    getSelectedSongs: () => [{ source: 'kuwo', url: '/download/1' }],
    isLocalMusicSourceValue: () => false,
    enqueueDownloadURLs: async () => { downloads++; return {added:1,existing:0}; },
    openDownloadRecordsModal: async () => {},
    setDownloadRecordsButtonState() {}, showToast: () => () => {},
    downloadNoticeDuration: () => 1000, refreshOpenDownloadRecords: async () => false,
    updateBatchToolbar() {},
    confirm() { throw new Error('native confirm must not be used'); },
  };
  vm.createContext(context);
  vm.runInContext(code, context);
  return { context, dialog: () => dialog, downloads: () => downloads, restored: () => restored };
}

test('confirmation starts downloads only after approval and prevents duplicate batches', async () => {
  const s = setup();
  const pending = s.context.batchDownload();
  assert.equal(s.dialog().open, true);
  assert.equal(s.downloads(), 0);
  await s.context.batchDownload();
  s.dialog().close('confirm');
  await pending;
  assert.equal(s.downloads(), 1);
  assert.equal(s.dialog().open, false);
  assert.equal(s.restored(), 1);
});

for (const reason of ['cancel', '']) {
  test(`cancel/Escape (${JSON.stringify(reason)}) sends no downloads and permits retry`, async () => {
    const s = setup();
    const pending = s.context.batchDownload();
    s.dialog().close(reason);
    await pending;
    assert.equal(s.downloads(), 0);
    const retry = s.context.batchDownload();
    assert.equal(s.dialog().open, true);
    s.dialog().close('confirm');
    await retry;
    assert.equal(s.downloads(), 1);
  });
}

test('entire 550-song batch is posted once and a later batch can be appended', async () => {
  const requests = [];
  const context = {
    API_ROOT: '/music', URL,
    crypto: require('node:crypto').webcrypto,
    window: { location: { href: 'http://127.0.0.1:37777/music/' } },
    loadDownloadQueue() {},
    fetch: async (url, options) => {
      requests.push({url, options, body:JSON.parse(options.body)});
      return {ok:true,json:async () => ({added:JSON.parse(options.body).songs.length,existing:0})};
    },
  };
  vm.createContext(context);
  const start = app.indexOf('async function enqueueDownloadURLs(');
  const end = app.indexOf('async function switchDownloadManagerTab(', start);
  vm.runInContext(app.slice(start,end),context);
  const urls = Array.from({length:550},(_,i) => `/music/download?id=${i}&source=kuwo&name=Song+${i}&artist=Artist&embed=1`);
  await context.enqueueDownloadURLs(urls);
  await context.enqueueDownloadURLs(['/music/download?id=new&source=kuwo&name=New']);
  assert.equal(requests.length,2);
  assert.equal(requests[0].body.songs.length,550);
  assert.equal(requests[0].body.songs[549].id,'549');
  assert.equal(requests[0].body.songs[0].embed,true);
  assert.equal(requests[0].options.headers['X-Requested-With'],'XMLHttpRequest');
  assert.notEqual(requests[0].body.request_id,requests[1].body.request_id);
});
