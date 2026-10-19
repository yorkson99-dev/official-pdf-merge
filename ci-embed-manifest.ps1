# Enable Windows Common Controls v6 for native TaskDialogIndirect.
# The assembly manifest is added to the same EXE resource section; it does
# not create an external *.manifest, and does not sign the executable.
$ErrorActionPreference = "Stop"
Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
public static class WinManifestResource {
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern IntPtr BeginUpdateResource(string file, bool deleteExisting);
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool UpdateResource(IntPtr handle, IntPtr resourceType, IntPtr resourceName, ushort language,
       byte[] data, uint length);
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool EndUpdateResource(IntPtr handle, bool discard);
    public static void AddManifest(string path) {
        string xml="<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>" +
           "<assembly xmlns=\"urn:schemas-microsoft-com:asm.v1\" manifestVersion=\"1.0\">" +
           "<assemblyIdentity version=\"1.0.0.0\" processorArchitecture=\"amd64\" name=\"OfficialPDFMerge\" type=\"win32\"/>" +
           "<dependency><dependentAssembly><assemblyIdentity type=\"win32\" name=\"Microsoft.Windows.Common-Controls\" " +
           "version=\"6.0.0.0\" processorArchitecture=\"*\" publicKeyToken=\"6595b64144ccf1df\" language=\"*\"/>" +
           "</dependentAssembly></dependency></assembly>";
        byte[] data=Encoding.UTF8.GetBytes(xml);
        IntPtr h=BeginUpdateResource(path,false);
        if(h==IntPtr.Zero)throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
        try {
            if(!UpdateResource(h,new IntPtr(24),new IntPtr(1),0x0409,data,(uint)data.Length))
                throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
        }catch {EndUpdateResource(h,true);throw;}
        if(!EndUpdateResource(h,false))
            throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
    }
}
'@
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
[WinManifestResource]::AddManifest($exe)
# Runtime visual tests verify the TaskDialog and progress control exist.
Write-Output 'PASS: Common Controls v6 assembly manifest embedded in the portable EXE'
