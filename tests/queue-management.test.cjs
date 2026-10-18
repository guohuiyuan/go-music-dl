const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const app = fs.readFileSync('internal/web/templates/static/js/app.js', 'utf8');

test('canceling clear does not send DELETE; confirming clears all pages and filters', async () => {
  const calls = [];
  let confirmed = false;
  const context = {
    API_ROOT:'/music', downloadQueuePage:8, downloadQueueRenderKey:'old',
    confirmAppAction:async()=>confirmed,
    fetch:async(url, options)=>{ calls.push([url,options]); return {ok:true,json:async()=>({status:'ok'})}; },
    loadDownloadQueue:async(page)=>calls.push(['refresh',page]), showToast(){}, downloadNoticeDuration:()=>1000,
  };
  vm.createContext(context);
  const start = app.indexOf('async function removeDownloadQueueTask(');
  vm.runInContext(app.slice(start,app.indexOf('window.setInterval',start)),context);
  await context.clearDownloadQueue();
  assert.equal(calls.length,0);
  confirmed = true;
  await context.clearDownloadQueue();
  assert.equal(calls[0][0],'/music/api/downloads/queue');
  assert.equal(calls[0][1].method,'DELETE');
  assert.equal(calls[0][1].headers['X-Requested-With'],'XMLHttpRequest');
  assert.equal(context.downloadQueuePage,1);
  assert.deepEqual(calls[1],['refresh',1]);
  await context.removeDownloadQueueTask(27);
  assert.equal(calls[2][0],'/music/api/downloads/queue/27');
});
