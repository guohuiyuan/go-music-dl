const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const app = fs.readFileSync('internal/web/templates/static/js/app.js','utf8');

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
