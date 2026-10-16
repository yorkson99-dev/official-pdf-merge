/* Public ZIP is read in the browser; Office conversions use an explicitly
 * selected local folder. No network requests, ActiveX in Edge, or EXE.
 * License: project code. Dependencies: JSZip(MIT), pdf-lib(MIT).
 */
(() => {
"use strict";
const SUPPORTED = new Set(["pdf","doc","docx","docm","odt","rtf","txt","xls","xlsx","xlsm","ods","csv","tsv","ppt","pptx","pptm","odp"]);
const TYPE_FOR = ext => ext==="pdf"?"PDF":(["doc","docx","docm","odt","rtf","txt"].includes(ext)?"Word":(["xls","xlsx","xlsm","ods","csv","tsv"].includes(ext)?"Excel":"PowerPoint"));
const MAX_ITEM=200*1024*1024, MAX_TOTAL=750*1024*1024, MAX_ENTRIES=4000;
const state={archive:null, ordered:[], bridge:null, ready:null, merging:false, header:null, mode:""};
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
function setRunning(v){state.merging=v;by("merge").disabled=v||!state.ordered.length;by("zipFile").disabled=v;by("filesInput").disabled=v;by("connect").disabled=v;by("clearFiles").disabled=v;}
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
async function fileBytes(item){
  if(item.entry)return item.entry.async("uint8array");
  if(item.file)return new Uint8Array(await item.file.arrayBuffer());
  throw Error("找不到來源檔案："+item.name);
}
function friendlyBridgeError(result){
  const [prefix,stage,file,code]=result.split("|");
  if(prefix!=="ERROR")return result;
  const stages={
    WORD_CREATE:"Word啟動失敗",WORD_MACRO_SECURITY:"Word巨集防護設定失敗",
    WORD_OPEN:"Word開啟文件失敗",WORD_EXPORT:"Word匯出PDF失敗",
    EXCEL_CREATE:"Excel啟動失敗",EXCEL_MACRO_SECURITY:"Excel巨集防護設定失敗",
    EXCEL_OPEN:"Excel開啟文件失敗",EXCEL_EXPORT:"Excel匯出PDF失敗",
    PPT_CREATE:"PowerPoint啟動失敗",PPT_MACRO_SECURITY:"PowerPoint巨集防護設定失敗",
    PPT_OPEN:"PowerPoint開啟文件失敗",PPT_EXPORT:"PowerPoint匯出PDF失敗",
    VERIFY_PDF:"已呼叫Office，但沒有產生有效PDF",
    INVALID_EXTENSION_OR_NAME:"橋接請求格式錯誤",MISSING_INPUT:"橋接找不到來源檔",
    MISSING_DIRECTORY:"工作資料夾缺失",REQUEST_NOT_READABLE:"橋接無法讀取請求",
    TOO_MANY_DOCUMENTS:"單次文件超過橋接限制"
  };
  return (stages[stage]||("橋接失敗："+stage))+
   (file?("（"+file+"）"):"")+(code?("；錯誤碼："+code):"")+
   "。請查看 Office 橋接命令視窗並記下錯誤碼。";
}
async function readJobText(dir,filename){
  try{const h=await dir.getFileHandle(filename);return (await (await h.getFile()).text()).trim();}
  catch(e){if(e.name==="NotFoundError")return "";throw e;}
}
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
      const bytes=await fileBytes(item);
      if(bytes.byteLength>MAX_ITEM)throw Error("附件超過200 MiB限制："+item.name);
      await writeFileHandle(input,name,bytes);
      request.push(name);
      indexToFile.set(i,pdf);
    }
    // The request is written last; the bridge never reads half-written input.
    await writeFileHandle(job,"request.txt",new TextEncoder().encode(request.join("\r\n")+"\r\n"));
    const started=Date.now(), timeout=8*60*1000;
    let lastStatus="",didWarn=false;
    while(Date.now()-started<timeout){
      await pause(650);
      const result=await readJobText(job,"result.txt");
      const status=await readJobText(job,"status.txt");
      if(status&&status!==lastStatus){
        lastStatus=status;
        const [stage,filename]=status.split("|");
        info("Office轉檔進度："+(stage||"處理中")+(filename?("／"+filename):"")+"\n工作編號："+jobName);
      }
      if(result){
        if(result!=="DONE")throw Error(friendlyBridgeError(result));
        const converted=new Map();
        for(const [i,pdf] of indexToFile){
          const b=await readFileHandle(output,pdf);
          if(new TextDecoder().decode(b.slice(0,5))!=="%PDF-")throw Error("Office輸出不是PDF："+ordered[i].name);
          converted.set(i,b);
        }
        return converted;
      }
      if(!status&&Date.now()-started>12000&&!didWarn){
        didWarn=true;
        info("已等候12秒但橋接器尚未接收請求。請確認「啟動 Office 橋接.cmd」有開啟、沒有報錯，且已選對 OfficeBridgeData 資料夾。\n工作編號："+jobName,true);
      }
    }
    throw Error("Office轉換等待8分鐘仍未完成；最後階段："+(lastStatus||"橋接器未接收請求")+"。請檢查腳本視窗／Office對話框。");
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
  if(!globalThis.PDFLib)throw Error("無法載入pdf-lib：請使用完整離線發行壓縮檔");
  const target=await globalThis.PDFLib.PDFDocument.create();
  let allPages=0;
  for(let i=0;i<ordered.length;i++){
    const item=ordered[i];
    onProgress(i,item.name);
    const bytes=item.ext==="pdf"?await fileBytes(item):converted.get(i);
    if(!bytes)throw Error("缺少Office轉檔結果："+item.name);
    if(bytes.length>MAX_ITEM)throw Error("單份PDF超過200 MiB限制："+item.name);
    let pdf;
    try{pdf=await globalThis.PDFLib.PDFDocument.load(bytes,{updateMetadata:false});}
    catch(e){throw Error("無法解析PDF「"+item.name+"」："+String(e.message||e))}
    const pages=pdf.getPageIndices();
    allPages+=pages.length;
    if(allPages>100000)throw Error("合併後頁數超過上限");
    const copied=await target.copyPages(pdf,pages);
    for(const page of copied)target.addPage(page);
  }
  return {bytes:await target.save({useObjectStreams:false}), pages:allPages};
}
function renderEntries(){
  const list=by("entries");list.replaceChildren();
  state.ordered.forEach((item,i)=>{
    const li=document.createElement("li");
    const seq=document.createElement("span");seq.textContent=String(i+1);
    const name=document.createElement("strong");name.textContent=item.name;
    const type=document.createElement("span");type.className="type";type.textContent=TYPE_FOR(item.ext);
    const controls=document.createElement("span");controls.className="list-controls";
    for(const [label,delta,title] of [["↑",-1,"向上"],["↓",1,"向下"],["×",null,"移除"]]){
      const btn=document.createElement("button");btn.type="button";btn.className="tiny";
      btn.textContent=label;btn.title=title+" "+item.name;btn.disabled=state.merging||(delta===-1&&i===0)||(delta===1&&i===state.ordered.length-1);
      btn.addEventListener("click",()=>{
        if(delta===null){state.ordered.splice(i,1);}
        else{[state.ordered[i],state.ordered[i+delta]]=[state.ordered[i+delta],state.ordered[i]];}
        state.ready=null;by("save").disabled=true;renderEntries();refreshDefaultName();
      });
      controls.append(btn);
    }
    li.append(seq,name,type,controls);list.append(li);
  });
  const count=state.ordered.length,needed=state.ordered.filter(v=>v.ext!=="pdf").length;
  by("inspection").textContent=count?("已加入"+count+"份文件，其中"+needed+"份需Office轉檔。可透過↑↓調整順序。"):"尚未選擇檔案";
  by("merge").disabled=state.merging||!count;
}
function refreshDefaultName(){
  let name="合併檔案.pdf";
  if(state.mode==="zip"){
    const header=state.header||{}, agency=safeName(header.agency||"");
    const full=by("detailedName").checked;
    if(header.agency){
      if(full&&header.date&&header.documentNo){
        name=safeName(agency+header.date+header.documentNo+(header.kind||"函")+
          (state.ordered.length>1?"及其附件":""))+".pdf";
      }else name=safeName(agency+"公文"+(state.ordered.length>1?"及其附件":""))+".pdf";
    }else{
      const stem=safeName((state.archive?.name||"公文").replace(/\\.zip$/i,""));
      name=stem+"_來文資料待確認.pdf";
    }
  }else if(state.ordered.length===1){
    name=safeName(state.ordered[0].name.replace(/\\.[^.]+$/,""))+".pdf";
  }
  by("outputName").value=name;
}
async function detectAgency(item){
  state.header=null;by("headerInfo").textContent="辨識主文抬頭中……";
  try{
    const text=await globalThis.OfficialHeader.extractHeaderFromPDF(await fileBytes(item));
    state.header=text;
    if(text.agency){
      by("headerInfo").textContent="來文機關："+text.agency+
        (text.date?"｜發文日期："+text.date:"")+
        (text.documentNo?"｜發文字號："+text.documentNo:"");
    }else by("headerInfo").textContent="主文PDF未辨識到機關名稱，請手動修改檔名（掃描PDF或特殊字型可能無法辨識）。";
  }catch(e){
    by("headerInfo").textContent="主文抬頭辨識失敗，仍可手動命名："+(e.message||e);
  }
}
async function inspect(file){
  if(!file)return;
  state.archive=null;state.ordered=[];state.ready=null;state.header=null;state.mode="zip";
  by("save").disabled=true;
  renderEntries();
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
    renderEntries();refreshDefaultName();
    info("已確認主文及附件排序。正在辨識機關名稱……");
    await detectAgency(ordered[0]);
    refreshDefaultName();
    info("已確認公文ZIP。可直接拖入其他文件追加，或按↑↓調整順序。");
  }catch(e){
    state.ordered=[];renderEntries();
    info(e.message||String(e),true);
  }
}
async function addFiles(files){
  const arr=Array.from(files||[]);
  if(!arr.length)return;
  if(arr.length===1&&extension(arr[0].name)==="zip"){await inspect(arr[0]);return;}
  if(arr.some(f=>extension(f.name)==="zip"))throw Error("ZIP和個別文件請分次加入，避免誤認附件順序");
  let total=0;
  for(const file of arr){
    const ext=extension(file.name);
    if(!SUPPORTED.has(ext))throw Error("不支援檔案："+file.name);
    if(file.size>MAX_ITEM)throw Error("單份附件超過200 MiB："+file.name);
    total+=file.size;
    if(total>MAX_TOTAL)throw Error("本次加入文件總大小超過750 MiB");
  }
  if(state.mode!=="zip")state.mode="manual";
  for(const file of arr)state.ordered.push({name:file.name,ext:extension(file.name),file});
  state.ready=null;by("save").disabled=true;renderEntries();
  if(state.mode==="manual"&&!state.header)by("headerInfo").textContent="手動加入模式：可直接修改合併檔名。";
  refreshDefaultName();
  info("已加入"+arr.length+"份文件，排序後可開始合併。");
}
function wireUI(){
  by("zipFile").addEventListener("change",e=>{inspect(e.target.files[0]);e.target.value="";});
  by("filesInput").addEventListener("change",async e=>{
    try{await addFiles(e.target.files);}catch(err){info(err.message||String(err),true);}
    finally{e.target.value="";}
  });
  const zone=by("dropZone");
  let dragDepth=0;
  zone.addEventListener("dragenter",event=>{event.preventDefault();dragDepth++;zone.classList.add("dragover");});
  zone.addEventListener("dragover",event=>{event.preventDefault();event.dataTransfer.dropEffect="copy";});
  zone.addEventListener("dragleave",event=>{event.preventDefault();dragDepth=Math.max(0,dragDepth-1);if(dragDepth===0)zone.classList.remove("dragover");});
  zone.addEventListener("drop",async event=>{
    event.preventDefault();dragDepth=0;zone.classList.remove("dragover");
    if(state.merging)return;
    try{await addFiles(event.dataTransfer.files);}catch(err){info(err.message||String(err),true);}
  });
  by("clearFiles").addEventListener("click",()=>{
    state.archive=null;state.ordered=[];state.ready=null;state.header=null;state.mode="";
    by("headerInfo").textContent="尚未辨識。";
    by("save").disabled=true;renderEntries();refreshDefaultName();info("清單已清空。");
  });
  by("detailedName").addEventListener("change",refreshDefaultName);

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
if(typeof module!=="undefined"&&module.exports)module.exports={parseZipEntries,safeName,isExcluded,extension,mergePdfDocuments,fileBytes,friendlyBridgeError};
if(typeof document!=="undefined")wireUI();
})();
