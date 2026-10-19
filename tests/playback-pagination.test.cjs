const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const code = fs.readFileSync('internal/web/templates/static/js/app.js','utf8');
function setup(fetch) {
  const ap = {on(){},list:{audios:[{original_id:'0',original_source:'local'}],add(songs){this.audios.push(...songs);}}};
  const ctx = {ap,fetch,API_ROOT:'/music',AbortController,URL,localMusicMatchCache:{},
    lyricURLsForPlayback:()=>({}),buildStreamURL:(id,source)=>`${source}:${id}`,showToast(){},
    localMusicSongFromTrack:track=>track,
    parsePositiveInt:(value,fallback)=>parseInt(value,10)||fallback,
    songFromCard:card=>card,
    DOMParser:class{parseFromString(text){const data=JSON.parse(text);return {querySelectorAll:()=>data.songs,querySelector:()=>({dataset:{totalPages:data.pages}})};}}
  };
  vm.createContext(ctx);
  vm.runInContext(code.slice(code.indexOf('let playbackExpansionController'),code.indexOf('function playAllAndJumpTo(btn)')),ctx);
  return ctx;
}
test('local playback loads beyond 200 and 500 tracks and deduplicates visible songs',async()=>{
  const offsets=[];
  const ctx=setup(async url=>{
    const offset=Number(new URL(url,'http://local').searchParams.get('offset')); offsets.push(offset);
    const tracks=Array.from({length:Math.min(500,2880-offset)},(_,i)=>({id:String(offset+i),source:'local',name:'song'}));
    return {ok:true,json:async()=>({tracks,offset,has_more:offset+tracks.length<2880})};
  });
  await ctx.expandPlaybackQueue({local:true});
  assert.deepEqual(offsets,[0,500,1000,1500,2000,2500]);
  assert.equal(ctx.ap.list.audios.length,2880);
});
test('online playback traverses all result pages and preserves equal IDs from different sources',async()=>{
  let requests=0;
  const ctx=setup(async url=>{requests++;const page=new URL(url).searchParams.get('page');return {ok:true,text:async()=>JSON.stringify({pages:2,songs:[{id:'same',source:page==='1'?'qq':'kuwo'}]})};});
  ctx.ap.list.audios=[];
  await ctx.expandPlaybackQueue({local:false,totalPages:2,url:'http://local/music/collection/1?page=8'});
  assert.equal(requests,2);
  assert.equal(ctx.ap.list.audios.length,2);
});
test('late response from an old playback queue cannot append to a new queue',async()=>{
  let release;
  const ctx=setup(()=>new Promise(resolve=>release=resolve));
  const pending=ctx.expandPlaybackQueue({local:true});
  ctx.cancelPlaybackExpansion();
  ctx.ap.list.audios=[{custom_id:'new',source:'qq'}];
  release({ok:true,json:async()=>({tracks:[{id:'old',source:'local'}],offset:0,has_more:false})});
  await pending;
  assert.equal(ctx.ap.list.audios.length,1);
  assert.equal(ctx.ap.list.audios[0].custom_id,'new');
});
test('page jump rejects invalid input and routes a valid page to its own handler',()=>{
  const calls=[];
  const ctx={goToPage:page=>calls.push(page)};vm.createContext(ctx);
  vm.runInContext(code.slice(code.indexOf('function submitPageJump'),code.indexOf('function goToPage(page')),ctx);
  const input={value:'7',reportValidity:()=>false};const form={querySelector:()=>input};const event={preventDefault(){}};
  ctx.submitPageJump(event,form);assert.equal(calls.length,0);
  input.reportValidity=()=>true;ctx.submitPageJump(event,form);assert.deepEqual(calls,[7]);
  ctx.submitPageJump(event,form,page=>calls.push(`queue:${page}`));assert.equal(calls[1],'queue:7');
});


test('clamped server offset stops rather than endlessly reloading the same batch', async()=>{
 let calls=0;
 const ctx=setup(async()=>{calls++;return {ok:true,json:async()=>({offset:1000,tracks:[],has_more:true})};});
 await ctx.expandPlaybackQueue({local:true});
 assert.equal(calls,1);
});
