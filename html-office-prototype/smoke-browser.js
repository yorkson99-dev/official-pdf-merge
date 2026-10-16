"use strict";
const assert=require("node:assert/strict");
const path=require("node:path");
const {pathToFileURL}=require("node:url");
const JSZip=require("jszip");
const {PDFDocument}=require("pdf-lib");
const {chromium}=require("playwright-core");

async function makePDF(width){
  const pdf=await PDFDocument.create();
  pdf.addPage([width,595]);
  return Buffer.from(await pdf.save());
}
async function main(){
  const zip=new JSZip();
  zip.file("來文/receipt_ATTCH2.pdf",await makePDF(320));
  zip.file("來文/receipt.pdf",await makePDF(210));
  const bytes=Buffer.from(await zip.generateAsync({type:"uint8array"}));
  const extra=await makePDF(380);
  const browser=await chromium.launch({channel:"msedge",headless:true});
  try{
    const page=await browser.newPage({acceptDownloads:true});
    const errors=[];
    page.on("pageerror",error=>errors.push(error.message));
    await page.goto(pathToFileURL(path.join(__dirname,"dist","index.html")).href);
    await page.waitForFunction(()=>Boolean(window.JSZip&&window.PDFLib&&window.OfficialHeader),null,{timeout:15000});
    const picker=await page.evaluate(()=>typeof window.showDirectoryPicker);
    console.log("Browser local HTML directory picker type:",picker);
    await page.locator("#zipFile").setInputFiles({
      name:"receipt.zip",mimeType:"application/zip",buffer:bytes
    });
    await page.waitForFunction(()=>document.querySelectorAll("#entries li").length===2
      && document.querySelector("#log").textContent.includes("已確認公文ZIP"),null,{timeout:20000});
    assert.match(await page.locator("#outputName").inputValue(),/來文資料待確認/);
    assert.equal(await page.locator("#merge").isEnabled(),true);
    // Exercise a genuine browser drop event and preserve the two ZIP entries.
    await page.evaluate(values=>{
      const transfer=new DataTransfer();
      transfer.items.add(new File([new Uint8Array(values)],"dragged.pdf",{type:"application/pdf"}));
      const zone=document.querySelector("#dropZone");
      zone.dispatchEvent(new DragEvent("drop",{dataTransfer:transfer,bubbles:true,cancelable:true}));
    },Array.from(extra));
    await page.waitForFunction(()=>document.querySelectorAll("#entries li").length===3,null,{timeout:10000});
    const third=await page.locator("#entries li").nth(2).textContent();
    assert.match(third,/dragged\.pdf/);
    // Reorder manual attachment into the second position.
    await page.locator("#entries li").nth(2).locator("button.tiny").first().click();
    const second=await page.locator("#entries li").nth(1).textContent();
    assert.match(second,/dragged\.pdf/);
    await page.locator("#merge").click();
    await page.waitForFunction(()=>document.querySelector("#save").disabled===false,null,{timeout:20000});
    assert.match(await page.locator("#log").textContent(),/總計3頁/);
    assert.deepEqual(errors,[],"Uncaught Edge browser errors: "+errors.join("; "));
    console.log("PASS: real Edge offline HTML, ZIP inspection, drag-drop, reordering and PDF merge");
  }finally{await browser.close();}
}
main().catch(e=>{console.error(e);process.exitCode=1;});
