/* Exercises the actual frontend controller with an in-memory DOM/API adapter.
 * No sockets, radio, webhooks or browser-layout claims. For real browser rendering
 * use ui.test.cjs separately. Only synthetic IDs and state are supplied here.
 */
'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

class Element {
  constructor() { this.children=[]; this.handlers={}; this.dataset={}; this.textContent=''; this.value=''; this.hidden=false; this.disabled=false; this.open=false; }
  append(...children) { this.children.push(...children.flatMap(c => c.fragment ? c.children : [c])); }
  replaceChildren(...children) { this.children=[]; this.append(...children); }
  setAttribute(name,value) { this[name]=value; }
  addEventListener(name,handler) { (this.handlers[name] ||= []).push(handler); }
  showModal() { this.open=true; }
  close() { this.open=false; }
  focus() {}
  reset() {}
  async click() { if(this.disabled || this.hidden) return; for(const handler of this.handlers.click || []) await handler({preventDefault(){},target:this}); }
}

async function harness() {
  const html=fs.readFileSync(path.join(__dirname,'static/index.html'),'utf8');
  const elements=new Map([...html.matchAll(/\bid="([^"]+)"/g)].map(m=>[m[1],new Element()]));
  const el=id=>{assert.ok(elements.has(id),`Missing real HTML element: ${id}`);return elements.get(id);};
  const button={id:'fixture-button',name:'A ON',enabled:false,signature:'',identity_fields:['codes'],webhook:{url:''}};
  let state={remotes:[{id:'fixture-remote',name:'Synthetic remote',columns:2,buttons:[button]}],settings:{},receiver:{state:'connected',message:'Synthetic samples'},events:[],version:'test',learning:null,learning_notice:null,actions_paused:false};
  const calls=[];
  let timerID=0; const timers=new Map();
  const session=()=>({session_id:'fixture-session',button_id:button.id,identity_fields:['codes'],automatic:true,stage:'waiting',message:'Receiver ready. Press once.',presses:0,required:3,candidate:'',expires_at:new Date(Date.now()+180000).toISOString(),diagnostics:{}});
  const context={document:{getElementById:el,createElement:()=>new Element(),createDocumentFragment:()=>Object.assign(new Element(),{fragment:true}),querySelectorAll:()=>[],documentElement:new Element()},window:{addEventListener(){}},sessionStorage:{getItem:()=> 'synthetic-test-token',setItem(){},removeItem(){}},localStorage:{getItem:()=>null,setItem(){}},AbortController,URL,Blob,Date,console,
    setTimeout:(fn,delay)=>{timers.set(++timerID,{fn,delay});return timerID;},clearTimeout:id=>timers.delete(id),
    fetch:async(url,options)=>{
      calls.push({url,method:options.method,body:options.body?JSON.parse(options.body):null});
      if(url==='/api/state') return {ok:true,json:async()=>structuredClone(state)};
      if(url==='/api/learn/start') state={...state,learning:session(),learning_notice:null,actions_paused:true};
      else if(url==='/api/learn/cancel') state={...state,learning:null,learning_notice:null,actions_paused:false};
      else if(url==='/api/learn/arm') state={...state,learning:{...state.learning,stage:'waiting'}};
      else if(url==='/api/learn/confirm') state={...state,learning:null,learning_notice:null,actions_paused:true};
      else throw Error(`Unexpected mutation: ${url}`);
      return {ok:true,json:async()=>({ok:true})};
    }};
  vm.runInNewContext(fs.readFileSync(path.join(__dirname,'static/app.js'),'utf8'),context);
  const flush=async()=>{for(let i=0;i<20;i++) await Promise.resolve();};
  await flush();
  const refresh=async()=>{const entry=[...timers.entries()].find(([,v])=>v.delay===1000);assert.ok(entry,'poll scheduled');timers.delete(entry[0]);await entry[1].fn();await flush();};
  return {el,calls,session,button,set:patch=>{state={...state,...patch};},refresh};
}

test('browser controller learns without decoder setup and exposes progress, diagnostics and retry',async()=>{
  const h=await harness();
  await h.el('button-grid').children[0].click();
  await h.el('learn-start').click();
  assert.equal(h.el('learning-dialog').open,true);
  assert.equal(h.el('learning-version').textContent,'RF Button Manager test');
  assert.match(h.el('learning-instruction').textContent,/Press the physical button once/);
  assert.equal(h.el('webhook-test').disabled,true);
  assert.deepEqual(h.calls.filter(c=>c.method==='POST').map(c=>c.url),['/api/learn/start']);
  for(const [stage,title] of [['receiving','Transmission detected'],['analyzing','Checking the captured signal']]) {
    h.set({learning:{...h.session(),stage,diagnostics:{packets:1,pulses:144,bytes:1441,candidates:1,replays:2}}});await h.refresh();
    assert.equal(h.el('learning-instruction').textContent,title);
    assert.match(h.el('learning-diagnostics').textContent,/pulses: 144/);
    assert.equal(h.el('learning-confirm').hidden,true);
  }
  h.set({learning:null,actions_paused:true,learning_notice:{...h.session(),stage:'unsupported',message:'Measured candidates did not decode consistently.'}});await h.refresh();
  assert.equal(h.el('learning-dialog').open,true,'failure must not silently close');
  assert.match(h.el('learning-description').textContent,/did not decode/);
  assert.equal(h.el('learning-retry').disabled,true,'wait for capture cleanup');
  h.set({actions_paused:false});await h.refresh();
  await h.el('learning-retry').click();
  assert.match(h.el('learning-count').textContent,/0 of 3/);
  assert.equal(h.el('learning-retry').hidden,true);
  assert.equal(h.calls.at(-2).url,'/api/learn/start');
  assert.equal(h.calls.at(-2).body.button_id,h.button.id);
  h.set({learning:{...h.session(),stage:'ready',presses:3,message:'Three captures verified.'}});await h.refresh();
  assert.equal(h.el('learning-confirm').hidden,false);
  await h.el('learning-confirm').click();
  assert.equal(h.el('learning-dialog').open,false);
  assert.equal(h.el('webhook-test').disabled,true,'restoration remains suppressed');
  assert.equal(h.calls.some(c=>c.url==='/api/settings' || c.url.endsWith('/test')),false);
});

test('reopened browser shows useful terminal failures and cannot retry before restoration',async()=>{
  const h=await harness(); // No selected editor: status controls must still update.
  for(const [stage,title] of [['receiver_unavailable','Receiver unavailable'],['timeout','Learning session timed out'],['press_timeout','Press window timed out'],['no_capture','No pulse capture'],['malformed_capture','Malformed pulse capture'],['unsupported_modulation','No supported modulation captured'],['capture_error','Capture stream failed'],['inconsistent','Captures did not agree']]) {
    h.set({learning_notice:{...h.session(),stage,message:`Explanation for ${stage}`},actions_paused:true});await h.refresh();
    assert.equal(h.el('learning-instruction').textContent,title);
    assert.equal(h.el('learning-retry').disabled,true);
    assert.equal(h.el('learning-expiry').textContent,'');
    assert.equal(h.el('learning-confirm').hidden,true);
  }
  await h.el('learning-cancel').click();
  assert.equal(h.el('learning-dialog').open,false);
  assert.equal(h.calls.at(-2).url,'/api/learn/cancel');
});

test('unrecognized traffic keeps the same press window visible with bounded analysis',async()=>{
  const h=await harness();
  const windowExpiry=new Date(Date.now()+20000).toISOString();
  const waiting={...h.session(),window_expires_at:windowExpiry,window_analyses:1,window_analysis_limit:4,message:'Unrecognized signal ignored; waiting for your button',diagnostics:{analysis_attempts:1,rejected_captures:1,recent_rejections:[{attempt:1,press_window:1,code:'unsupported',reason:'No candidate in this recording',packets:1,bytes:72,candidates:0,replays:2}]}};
  h.set({learning:waiting,actions_paused:true});await h.refresh();
  assert.equal(h.el('learning-description').textContent,waiting.message);
  assert.match(h.el('learning-count').textContent,/0 of 3/);
  assert.match(h.el('learning-expiry').textContent,/Press window: \d+s · Session limit:/);
  assert.match(h.el('learning-diagnostics').textContent,/Capture 1, press window 1: unsupported/);
  assert.equal(h.el('learning-confirm').hidden,true);
  assert.equal(h.el('learning-arm').hidden,true);
  assert.equal(h.el('learning-retry').hidden,true);
  h.set({learning:{...waiting,window_analyses:4,message:'Background traffic used this window’s analysis budget.'}});await h.refresh();
  assert.equal(h.el('learning-instruction').textContent,'Analysis budget reached');
  assert.equal(h.el('learning-confirm').hidden,true);
  assert.equal(h.calls.some(c=>c.method==='POST'),false,'polling must never automatically rearm or confirm');
});

test('failure diagnostics retain capture evidence separately from restored reception',async()=>{
  const h=await harness();
  h.set({receiver_mode:'normal',receiver:{state:'connected',message:'Restored samples'},learning_notice:{...h.session(),stage:'malformed_capture',capture_generation:7,capture_receiver:{state:'error',message:'Capture pipe failed'},capture_receiver_at:'2026-01-01T00:00:00Z',diagnostics:{observed_packets:1,packets:0,rejected_packets:1,pulses:12,bytes:321,malformed_packets:1,failure_code:'invalid_timing',failure_reason:'Pulse row 13 contains a non-integer or negative duration',stream:{bytes:400,lines:20,json_records:1,headers:1,completed:1,closed:true}}}});
  await h.refresh();
  const text=h.el('learning-diagnostics').textContent;
  assert.match(text,/Capture receiver at failure: error — Capture pipe failed/);
  assert.match(text,/Current reception: normal reception — connected — Restored samples/);
  assert.match(text,/Packet records seen: 1; accepted for replay: 0; rejected: 1/);
  assert.match(text,/Observed pulses: 12; packet bytes: 321/);
  assert.match(text,/Stdout read: 400 bytes \/ 20 lines; JSON records: 1/);
  assert.match(text,/Failure: invalid_timing — Pulse row 13/);
  assert.match(text,/Pulse stream: closed/);
  assert.equal(h.el('learning-instruction').textContent,'Malformed pulse capture');
  assert.equal(h.el('learning-confirm').hidden,true);
});
