"use strict";
const assert=require("node:assert/strict");
const JSZip=require("jszip");
const {PDFDocument}=require("pdf-lib");
globalThis.PDFLib={PDFDocument};
const {parseZipEntries,mergePdfDocuments,safeName,isExcluded,fileBytes,friendlyBridgeError}=require("./app.js");
const {extractHeaderFromLines,groupTextItems}=require("./header.js");

async function makePDF(width){
  const pdf=await PDFDocument.create();
  pdf.addPage([width,595]);
  return await pdf.save();
}
async function run(){
  assert.equal(safeName("a<b>.pdf"),"ab.pdf");
  const header=extractHeaderFromLines([
    "臺北市政府地政局  函",
    "受文者：○○地政事務所",
    "發文日期：中華民國115年10月16日",
    "發文字號：北市地測字第1156012345號",
    "主旨：本案測試",
  ]);
  assert.deepEqual(header,{
    agency:"臺北市政府地政局",kind:"函",
    date:"115年10月16日",documentNo:"北市地測字第1156012345號"
  });
  const rows=groupTextItems([
    {str:"臺北市政府地政局",transform:[1,0,0,1,100,700]},
    {str:"函",transform:[1,0,0,1,220,700]},
    {str:"發文日期：115年10月16日",transform:[1,0,0,1,100,660]}
  ]);
  assert.match(rows[0],/臺北市政府地政局.*函/);
  assert.match(friendlyBridgeError("ERROR|WORD_OPEN|0001.odt|-2147221005"),/Word開啟文件失敗.*0001.odt.*-2147221005/);
  const fake={name:"appended.pdf",file:{arrayBuffer:async()=>new Uint8Array([37,80,68,70]).buffer}};
  assert.equal((await fileBytes(fake))[0],37);
  console.log("PASS: official agency/date/number extraction and precise Office COM diagnostics");
  assert.equal(isExcluded("x(opinion).pdf"),true);
  const zip=new JSZip();
  zip.file("來文/115_11_ATTCH10.pdf",await makePDF(350));
  zip.file("來文/115_11_ATTCH2.odt","fake Office input");
  zip.file("來文/115_11.pdf",await makePDF(200));
  zip.file("來文/115_11_ATTCH1.pdf",await makePDF(280));
  zip.file("來文/115_11(opinion).pdf",await makePDF(900));
  zip.file("稿/ignore.pdf",await makePDF(950));
  const encoded=await zip.generateAsync({type:"uint8array"});
  const opened=await JSZip.loadAsync(encoded);
  const ordered=parseZipEntries(opened);
  assert.deepEqual(ordered.map(x=>x.name),[
    "115_11.pdf","115_11_ATTCH1.pdf","115_11_ATTCH2.odt","115_11_ATTCH10.pdf"
  ]);
  const converted=new Map([[2,await makePDF(310)]]);
  const result=await mergePdfDocuments(ordered,converted,()=>{});
  assert.equal(result.pages,4);
  const output=await PDFDocument.load(result.bytes);
  assert.deepEqual(output.getPages().map(page=>Math.round(page.getWidth())),[200,280,310,350]);
  console.log("PASS: ZIP main/ATTCH ordering, Office PDF placeholder and browser merge output");

  const broken=new JSZip();
  broken.file("來文/base.pdf",await makePDF(200));
  broken.file("來文/base_ATTCH1.exe","not allowed");
  assert.throws(()=>parseZipEntries(broken),/不支援/);
  const duplicate=new JSZip();
  duplicate.file("來文/base.pdf",await makePDF(200));
  duplicate.file("來文/base_ATTCH1.pdf",await makePDF(200));
  duplicate.file("來文/base_ATTACH1.pdf",await makePDF(200));
  assert.throws(()=>parseZipEntries(duplicate),/重複/);
  console.log("PASS: unsafe attachments rejected, duplicate ATTCH numbers rejected");
}
run().catch(e=>{console.error(e);process.exitCode=1});
