const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const app = fs.readFileSync('internal/web/templates/static/js/app.js','utf8');

function deletionContext(confirmed,ok=true) {
  const calls=[];
  const context={
    API_ROOT:'/music',URL,
    window:{location:{href:'http://127.0.0.1:37777/music/my_collections'}},
    confirmAppAction:async options=>{calls.push(['confirm',options]);return confirmed;},
    confirm(){throw new Error('native confirm is unavailable');},
    fetch:async (url,options)=>{calls.push(['fetch',url,options]);return {ok,json:async()=>ok?{status:'ok'}:{error:'数据库暂不可用'}};},
    refreshCurrentPageContent:async()=>{calls.push(['refresh']);},
    refreshAddToCollectionList:()=>{calls.push(['modal-refresh']);},
    showToast:(...args)=>{calls.push(['toast',...args]);},downloadNoticeDuration:()=>1000,
  };
  vm.createContext(context);
  vm.runInContext(app.slice(app.indexOf('let collectionDeletePending'),app.indexOf('function refreshAddToCollectionList()')),context);
  return {context,calls};
}

test('canceling deletion leaves collection untouched',async()=>{
  const {context,calls}=deletionContext(false);
  await context.deleteCollection(8);
  assert.equal(calls.filter(c=>c[0]==='fetch').length,0);
});
test('confirmed modal deletion sends guarded DELETE and refreshes both views',async()=>{
  const {context,calls}=deletionContext(true);
  await context.deleteCollectionFromModal(8);
  const request=calls.find(c=>c[0]==='fetch');
  assert.equal(request[1],'/music/collections/8');
  assert.equal(request[2].method,'DELETE');
  assert.equal(request[2].headers['X-Requested-With'],'XMLHttpRequest');
  assert.ok(calls.some(c=>c[0]==='refresh'));
  assert.ok(calls.some(c=>c[0]==='modal-refresh'));
  assert.ok(calls.some(c=>c[0]==='toast'&&c[1]==='歌单已删除'));
});
test('failed deletion shows error, does not refresh away the collection, and permits retry',async()=>{
  const {context,calls}=deletionContext(true,false);
  await context.deleteCollection(8);
  await context.deleteCollection(8);
  assert.equal(calls.filter(c=>c[0]==='fetch').length,2);
  assert.equal(calls.filter(c=>c[0]==='refresh').length,0);
  assert.ok(calls.some(c=>c[0]==='toast'&&c[1]==='删除歌单失败'&&c[2]==='数据库暂不可用'));
});
test('whole-album button submits container identity rather than visible song cards',async()=>{
  const requests=[];
  const button={disabled:false,innerHTML:'下载整张专辑',dataset:{containerKind:'album',containerId:'album-500',containerSource:'kuwo'}};
  const context={
    API_ROOT:'/music',crypto:require('node:crypto').webcrypto,webSettings:{embedDownload:true},
    fetch:async(url,options)=>{requests.push([url,JSON.parse(options.body)]);return {ok:true,json:async()=>({total:125,added:125,existing:0,skipped_local:0})};},
    getSelectedSongs(){throw new Error('whole-container download must not depend on page selection');},
    showToast(){},downloadNoticeDuration:()=>1000,openDownloadRecordsModal:async()=>{},
  };
  vm.createContext(context);
  const start=app.indexOf('async function downloadWholeCollection(');
  vm.runInContext(app.slice(start,app.indexOf('function importCollectionFromButton(',start)),context);
  await context.downloadWholeCollection(button);
  assert.equal(requests[0][0],'/music/api/downloads/queue/container');
  assert.equal(requests[0][1].id,'album-500');
  assert.equal(requests[0][1].kind,'album');
  assert.equal(requests[0][1].embed,true);
  assert.equal(button.disabled,false);
  assert.equal(button.innerHTML,'下载整张专辑');
});
