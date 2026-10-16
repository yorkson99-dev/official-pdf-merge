/* Public ZIP is read in the browser; Office conversions use an explicitly
 * selected local folder. No network requests, ActiveX in Edge, or EXE.
 * License: project code. Dependencies: JSZip(MIT), pdf-lib(MIT).
 */
(() => {
"use strict";
const SUPPORTED = new Set(["pdf","doc","docx","docm","odt","rtf","txt","xls","xlsx","xlsm","ods","csv","tsv","ppt","pptx","pptm","odp"]);
const TYPE_FOR = ext => ext==="pdf"?"PDF":(["doc","docx","docm","odt","rtf","txt"].includes(ext)?"Word":(["xls","xlsx","xlsm","ods","csv","tsv"].includes(ext)?"Excel":"PowerPoint"));
const MAX_ITEM=200*1024*1024, MAX_TOTAL=750*1024*1024, MAX_ENTRIES=4000;
const state={archive:null, ordered:[], bridge:null, ready:null, merging:false};
const by=id=>document.getElementById(id);
function safeName(raw){
  const s=String(raw).replace(/[\\/:*?"<>|\x00-\x1f]/g,"").replace(/[. ]+$/g,"").trim();
  return s.slice(0,150)||"合併檔案";
}
function isExcluded(s){
  return /opinion|\(after\)|\(before\)|合併版|意見單|\(合併\)/i.test(s);
}
function normalize(name){return name.replace(/\\/g,"/");}
function extension(name){const pos=name.lastIndexOf(".");return pos>=0?name.slice(pos+1).toLowerCase():"";}
function parseZipEntries(zip){
  const candidates=[], attachments=new Map();
  const all=Object.values(zip.files);
  if(all.length>MAX_ENTRIES)throw Error("ZIP項目過多");
  for(const entry of all){
    if(entry.dir)continue;
    const parts=normalize(entry.name).split("/");
    if(parts.length!==2||parts[0]!=="來文")continue;
    const name=parts[1];if(!name||isExcluded(name))continue;
    const ext=extension(name);
    const m=/^(.*?)_(?:ATTCH|ATTACH)(\d+)\.[^.]+$/i.exec(name);
    if(m){
      if(!SUPPORTED.has(ext))throw Error("附件副檔名不支援："+name);
      const n=Number(m[2]);
      if(!Number.isSafeInteger(n)||n<1)throw Error("附件編號異常："+name);
      const key=m[1].toLowerCase();
      const a=attachments.get(key)||[];
      a.push({name,ext,n,entry});
      attachments.set(key,a);
    }else if(ext==="pdf"){
      const key=name.slice(0,-4).toLowerCase();
      candidates.push({key,name,ext,n:0,entry});
    }
  }
  if(candidates.length!==1)throw Error("來文資料夾必須剛好包含一份主文PDF，目前找到"+candidates.length+"份");
  const main=candidates[0];
  const att=(attachments.get(main.key)||[]).sort((a,b)=>a.n-b.n);
  for(let i=1;i<att.length;i++)if(att[i].n===att[i-1].n)throw Error("ATTCH編號重複");
  const result=[main,...att];
  let total=0;
  for(const v of result){
    const size=v.entry._data&&v.entry._data.uncompressedSize;
    if(Number.isFinite(size)){
      if(size>MAX_ITEM)throw Error("附件超過單份200 MiB限制："+v.name);
      total+=size;
      if(total>MAX_TOTAL)throw Error("附件合計超過750 MiB限制");
    }
  }
  return result;
}
function info(msg, isError=false){const el=by("log");el.textContent=msg;el.classList.toggle("error",isError);}
function setRunning(v){state.merging=v;by("merge").disabled=v||!state.ordered.length;by("zipFile").disabled=v;by("connect").disabled=v;}
async function readFileHandle(dir,filename){
  const h=await dir.getFileHandle(filename);
  const file=await h.getFile();
  return new Uint8Array(await file.arrayBuffer());
}
async function writeFileHandle(dir,filename,data){
  const h=await dir.getFileHandle(filename,{create:true});
  const writer=await h.createWritable();
  try{await writer.write(data);}finally{await writer.close();}
}
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
async function callOfficeBridge(ordered){
  if(!state.bridge)throw Error("尚未連線：請先選擇OfficeBridgeData資料夾");
  const jobName="j"+crypto.randomUUID().replace(/-/g,"");
  const jobs=await state.bridge.getDirectoryHandle("jobs",{create:true});
  const job=await jobs.getDirectoryHandle(jobName,{create:true});
  const input=await job.getDirectoryHandle("input",{create:true});
  const output=await job.getDirectoryHandle("output",{create:true});
  const request=[];
  const indexToFile=new Map();
  try{
    for(let i=0;i<ordered.length;i++){
      const item=ordered[i];
      if(item.ext==="pdf")continue;
      const name=String(i).padStart(4,"0")+"."+item.ext;
      const pdf=name.slice(0,-item.ext.length)+"pdf";
      const bytes=await item.entry.async("uint8array");
      if(bytes.byteLength>MAX_ITEM)throw Error("附件超過200 MiB限制："+item.name);
      await writeFileHandle(input,name,bytes);
      request.push(name);
      indexToFile.set(i,pdf);
    }
    // The request is written last; the bridge never reads half-written input.
    await writeFileHandle(job,"request.txt",new TextEncoder().encode(request.join("\r\n")+"\r\n"));
    const started=Date.now(), timeout=8*60*1000;
    while(Date.now()-started<timeout){
      await pause(600);
      let result=null;
      try{
        const h=await job.getFileHandle("result.txt");
        result=(await (await h.getFile()).text()).trim();
      }catch(e){if(e.name!=="NotFoundError")throw e;}
      if(result){
        if(result!=="DONE")throw Error("Office橋接失敗："+result);
        const converted=new Map();
        for(const [i,pdf] of indexToFile){
          const b=await readFileHandle(output,pdf);
          if(new TextDecoder().decode(b.slice(0,5))!=="%PDF-")throw Error("Office輸出不是PDF："+ordered[i].name);
          converted.set(i,b);
        }
        return converted;
      }
    }
    throw Error("Office轉換等待超時：請檢查橋接視窗是否仍開啟，並留意Office對話框");
  }finally{
    // Do not remove the workspace on timeout: bridge might still be working.
    // Successful and explicit failure results are safe to clean when inactive.
    try{
      const h=await job.getFileHandle("result.txt");
      if(h)await jobs.removeEntry(jobName,{recursive:true});
    }catch(e){/* Leave timed-out job for manual review; never delete its inputs mid-conversion. */}
  }
}
async function mergePdfDocuments(ordered,converted,onProgress){
  if(!window.PDFLib)throw Error("無法載入pdf-lib：請使用完整離線發行壓縮檔");
  const target=await PDFLib.PDFDocument.create();
  let allPages=0;
  for(let i=0;i<ordered.length;i++){
    const item=ordered[i];
    onProgress(i,item.name);
    const bytes=item.ext==="pdf"?await item.entry.async("uint8array"):converted.get(i);
    if(!bytes)throw Error("缺少Office轉檔結果："+item.name);
    if(bytes.length>MAX_ITEM)throw Error("單份PDF超過200 MiB限制："+item.name);
    let pdf;
    try{pdf=await PDFLib.PDFDocument.load(bytes,{updateMetadata:false});}
    catch(e){throw Error("無法解析PDF「"+item.name+"」："+String(e.message||e))}
    const pages=pdf.getPageIndices();
    allPages+=pages.length;
    if(allPages>100000)throw Error("合併後頁數超過上限");
    const copied=await target.copyPages(pdf,pages);
    for(const page of copied)target.addPage(page);
  }
  return {bytes:await target.save({useObjectStreams:false}), pages:allPages};
}
async function inspect(file){
  state.archive=null;state.ordered=[];state.ready=null;by("save").disabled=true;by("entries").replaceChildren();
  if(!file)return;
  try{
    if(!window.JSZip)throw Error("無法載入JSZip：請使用完整離線發行壓縮檔");
    if(file.size>MAX_TOTAL)throw Error("ZIP檔案超過750 MiB限制");
    info("分析ZIP結構中……");
    const zip=await JSZip.loadAsync(file,{
      checkCRC32:true,
      decodeFileName:bytes=>new TextDecoder("big5").decode(bytes)
    });
    const ordered=parseZipEntries(zip);
    state.archive=file;state.ordered=ordered;
    for(let i=0;i<ordered.length;i++){
      const item=ordered[i];
      const li=document.createElement("li");
      const seq=document.createElement("span");seq.textContent=String(i+1);
      const filename=document.createElement("strong");filename.textContent=item.name;
      const type=document.createElement("span");type.className="type";type.textContent=TYPE_FOR(item.ext);
      li.append(seq,filename,type);by("entries").append(li);
    }
    by("inspection").textContent="已辨識"+ordered.length+"份文件，其中"+ordered.filter(i=>i.ext!=="pdf").length+"份需Microsoft Office轉檔。";
    const stem=safeName(file.name.replace(/\.zip$/i,""));
    by("outputName").value=stem+(ordered.length>1?"_公文及其附件.pdf":"_公文.pdf");
    by("merge").disabled=false;
    info("已確認公文主文及附件排序，可開始處理。");
  }catch(e){
    by("inspection").textContent="無法辨識公文ZIP";
    by("merge").disabled=true;info(e.message||String(e),true);
  }
}
function wireUI(){
  by("zipFile").addEventListener("change",e=>{inspect(e.target.files[0]);});
  by("connect").addEventListener("click",async()=>{
    if(!window.showDirectoryPicker){
      by("bridgeState").textContent="Edge未提供資料夾存取API；請確認瀏覽器及執行政策";
      return;
    }
    try{
      const h=await showDirectoryPicker({mode:"readwrite",id:"official-pdf-bridge"});
      if(h.name!=="OfficeBridgeData")throw Error("請選擇套件內的OfficeBridgeData資料夾");
      await h.getDirectoryHandle("jobs",{create:true});
      state.bridge=h;by("bridgeState").textContent="已連接本機橋接資料夾（不代表轉檔腳本已啟動）";
      by("bridgeState").className="ok";
    }catch(e){if(e.name!=="AbortError"){by("bridgeState").textContent=e.message;by("bridgeState").className="error";}}
  });
  by("merge").addEventListener("click",async()=>{
    if(!state.ordered.length)return;
    state.ready=null;by("save").disabled=true;setRunning(true);
    try{
      const needsOffice=state.ordered.some(x=>x.ext!=="pdf");
      info(needsOffice?"正在等待Office橋接轉檔……":"正在合併PDF……");
      const converted=needsOffice?await callOfficeBridge(state.ordered):new Map();
      const result=await mergePdfDocuments(state.ordered,converted,(i,n)=>info("合併PDF："+(i+1)+"/"+state.ordered.length+"\n"+n));
      state.ready=result;
      info("合併完成："+state.ordered.length+"份文件，總計"+result.pages+"頁。請按「儲存合併PDF」。");
      by("save").disabled=false;
    }catch(e){info(e.message||String(e),true);}
    finally{setRunning(false);}
  });
  by("save").addEventListener("click",async()=>{
    if(!state.ready)return;
    const name=safeName(by("outputName").value.replace(/\.pdf$/i,""))+".pdf";
    const blob=new Blob([state.ready.bytes],{type:"application/pdf"});
    if(window.showSaveFilePicker){
      try{
        // Called synchronously from the user click, as required by Chromium.
        const handle=await window.showSaveFilePicker({suggestedName:name,types:[{description:"PDF",accept:{"application/pdf":[".pdf"]}}]});
        const writer=await handle.createWritable();
        try{await writer.write(blob);}finally{await writer.close();}
        info("已儲存："+name+"（"+state.ready.pages+"頁）");
        return;
      }catch(e){
        if(e.name==="AbortError")return;
        info("另存視窗不可用，改用瀏覽器下載："+(e.message||e));
      }
    }
    const url=URL.createObjectURL(blob);
    const a=document.createElement("a");a.href=url;a.download=name;document.body.append(a);a.click();a.remove();
    setTimeout(()=>URL.revokeObjectURL(url),30000);
  });
}
if(typeof module!=="undefined"&&module.exports)module.exports={parseZipEntries,safeName,isExcluded,extension};
if(typeof document!=="undefined")wireUI();
})();
