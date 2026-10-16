/* Extract official correspondence header from the original, searchable main
 * PDF using bundled pdf.js. No OCR, upload, external fonts or internet access.
 */
(() => {
"use strict";
function clean(s){return String(s||"").replace(/[\u200b-\u200f\ufeff]/g,"").replace(/[\t　]+/g," ").trim();}
function extractHeaderFromLines(lines){
  const cleanLines=lines.map(clean).filter(Boolean);
  // Avoid interpreting subject paragraphs, addressees or attachment lists as
  // issuing-agency titles; look only in the top of the first page.
  let agency="",kind="";
  const candidates=cleanLines.slice(0,16);
  for(let i=0;i<candidates.length;i++){
    const line=candidates[i].replace(/\s+/g," ").trim();
    const match=/^(.{2,70}?)\s*(開會通知單|書函|通知|函)$/.exec(line);
    if(!match)continue;
    const name=match[1].replace(/\s/g,"").trim();
    if(!/[\u3400-\u9fff]/.test(name)||/受文者|發文字號|發文日期|主旨|密等|附件/.test(name))continue;
    agency=name;kind=match[2];break;
  }
  // PDF renderers can put agency and document type in separate positioned
  // text rows. Only join adjacent short heading rows, never body paragraphs.
  if(!agency){
    for(let i=0;i<candidates.length-1;i++){
      const title=candidates[i].replace(/\s+/g,"").trim();
      const next=candidates[i+1].replace(/\s+/g,"").trim();
      if(/^(開會通知單|書函|通知|函)$/.test(next)
          && /^[\u3400-\u9fff\w（）()－\-]{2,50}$/.test(title)
          && !/受文者|發文字號|發文日期|主旨|密等|附件/.test(title)){
        agency=title;kind=next;break;
      }
    }
  }
  const joined=cleanLines.join("\n").replace(/[ \t　]*([：:])[ \t　]*/g,"$1");
  // The raw text may separate a short label and its value into adjacent
  // lines, so test both normal lines and a compact join of the first page.
  const compact=cleanLines.join(" ").replace(/\s+/g," ");
  const date=/(?:發文日期)\s*[：:]?\s*(?:中華民國)?\s*(\d{2,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日/.exec(compact);
  let number=/(?:發文字號)\s*[：:]?\s*([^\n]{1,70}?號)/.exec(joined);
  if(!number)number=/(?:發文字號)\s*[：:]?\s*([^\n]{1,70}?號)/.exec(compact);
  const docno=number?number[1].replace(/\s+/g,"").trim():"";
  return {agency,kind:kind||"函",date:date?date[1]+"年"+date[2]+"月"+date[3]+"日":"",documentNo:docno};
}
function groupTextItems(items){
  const rows=[];
  for(const item of items){
    if(!item.str)continue;
    const y=Number(item.transform?.[5]||0),x=Number(item.transform?.[4]||0);
    let row=rows.find(r=>Math.abs(r.y-y)<2.5);
    if(!row){row={y,items:[]};rows.push(row);}
    row.items.push({x,text:item.str});
  }
  rows.sort((a,b)=>b.y-a.y);
  return rows.map(row=>row.items.sort((a,b)=>a.x-b.x).map(item=>item.text).join(" ")).filter(s=>s.trim());
}
async function extractHeaderFromPDF(bytes){
  if(!globalThis.pdfjsLib)throw Error("找不到離線PDF文字辨識函式庫");
  if(globalThis.pdfjsLib.GlobalWorkerOptions)
    globalThis.pdfjsLib.GlobalWorkerOptions.workerSrc="vendor/pdf.worker.min.js";
  const task=globalThis.pdfjsLib.getDocument({
    data:bytes,
    isEvalSupported:false,
    useSystemFonts:true,
    stopAtErrors:true
  });
  let doc;
  try{
    doc=await task.promise;
    if(doc.numPages<1)return {agency:"",date:"",documentNo:"",kind:"函"};
    const page=await doc.getPage(1);
    const content=await page.getTextContent();
    return extractHeaderFromLines(groupTextItems(content.items));
  }finally{
    if(doc)await doc.destroy();
    else await task.destroy();
  }
}
const api={extractHeaderFromLines,extractHeaderFromPDF,groupTextItems};
if(typeof module!=="undefined"&&module.exports)module.exports=api;
if(typeof globalThis!=="undefined")globalThis.OfficialHeader=api;
})();
