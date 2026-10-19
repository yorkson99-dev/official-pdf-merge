# Embed native Windows VERSIONINFO resource so Explorer Properties > Details
# shows the same attribution visible in the Win32 interface.
# This is developer attribution, NOT an Authenticode digital signature.
$ErrorActionPreference = "Stop"
Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Text;
using System.Runtime.InteropServices;
public static class OfficialVersionResource {
    [DllImport("kernel32.dll", CharSet=CharSet.Unicode, SetLastError=true)]
    static extern IntPtr BeginUpdateResource(string file, bool deleteExisting);
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool UpdateResource(IntPtr handle, IntPtr resourceType, IntPtr resourceName,
        ushort language, byte[] data, uint length);
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool EndUpdateResource(IntPtr handle, bool discard);

    static void Align(BinaryWriter w) {
        while ((w.BaseStream.Position & 3) != 0) w.Write((byte)0);
    }
    static byte[] Block(string key, byte[] value, ushort valueLength, ushort type, params byte[][] children) {
        using (var stream = new MemoryStream()) {
            using (var w = new BinaryWriter(stream, Encoding.Unicode, true)) {
                w.Write((ushort)0); // wLength, patched below
                w.Write(valueLength);
                w.Write(type);
                w.Write(Encoding.Unicode.GetBytes(key + "\0"));
                Align(w);
                w.Write(value);
                Align(w);
                foreach (var child in children) {
                    w.Write(child);
                }
                Align(w);
                if (stream.Length > ushort.MaxValue) throw new Exception("VERSIONINFO too long");
                var block = stream.ToArray();
                BitConverter.GetBytes((ushort)block.Length).CopyTo(block, 0);
                return block;
            }
        }
    }
    static byte[] VersionString(string key, string value) {
        return Block(key, Encoding.Unicode.GetBytes(value + "\0"),
            checked((ushort)(value.Length + 1)), 1);
    }
    public static byte[] Build() {
        byte[][] entries = {
            VersionString("CompanyName", "臺北市政府地政局測繪科"),
            VersionString("FileDescription", "公文 PDF 快合"),
            VersionString("FileVersion", "1.6.3.0"),
            VersionString("InternalName", "OfficialPDFMerge"),
            VersionString("LegalCopyright", "開發：臺北市政府地政局測繪科"),
            VersionString("OriginalFilename", "OfficialPDFMerge.exe"),
            VersionString("ProductName", "公文 PDF 快合"),
            VersionString("ProductVersion", "1.6.3.0"),
            VersionString("Comments", "臺北市政府地政局測繪科開發"),
        };
        byte[] table = Block("040904B0", Array.Empty<byte>(), 0, 1, entries);
        byte[] stringInfo = Block("StringFileInfo", Array.Empty<byte>(), 0, 1, table);
        // 0x0409 = en-US, 0x04b0 = 1200 Unicode.
        byte[] translation = Block("Translation", new byte[]{0x09, 0x04, 0xB0, 0x04}, 4, 0);
        byte[] varInfo = Block("VarFileInfo", Array.Empty<byte>(), 0, 1, translation);
        byte[] fixedValue;
        using (var ms = new MemoryStream()) {
            using (var w = new BinaryWriter(ms, Encoding.Unicode, true)) {
                uint[] values = {
                    0xFEEF04BDu, 0x00010000u,
                    (1u << 16) | 6u, (3u << 16),
                    (1u << 16) | 6u, (3u << 16),
                    0x3Fu, 0u, 0x00040004u, 1u, 0u, 0u, 0u
                };
                foreach (var value in values) w.Write(value);
                fixedValue = ms.ToArray();
            }
        }
        return Block("VS_VERSION_INFO", fixedValue, (ushort)fixedValue.Length, 0, stringInfo, varInfo);
    }
    public static void Embed(string path) {
        byte[] data = Build();
        IntPtr handle = BeginUpdateResource(path, false);
        if (handle == IntPtr.Zero) throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
        try {
            // RT_VERSION = 16, resource 1, 0x0409.
            if (!UpdateResource(handle, new IntPtr(16), new IntPtr(1), 0x0409, data, (uint)data.Length))
                throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
        } catch {
            EndUpdateResource(handle, true);
            throw;
        }
        if (!EndUpdateResource(handle, false))
            throw new System.ComponentModel.Win32Exception(Marshal.GetLastWin32Error());
    }
}
'@
$exe=Join-Path $PSScriptRoot '公文PDF快合.exe'
[OfficialVersionResource]::Embed($exe)
$version=[System.Diagnostics.FileVersionInfo]::GetVersionInfo($exe)
$expected='臺北市政府地政局測繪科'
if ($version.CompanyName -ne $expected) { throw "CompanyName mismatch: $($version.CompanyName)" }
if ($version.Comments -ne ($expected + '開發')) { throw "Comments mismatch: $($version.Comments)" }
if ($version.FileVersion -ne '1.6.3.0') { throw "FileVersion mismatch: $($version.FileVersion)" }
Write-Output "PASS: Windows VERSIONINFO CompanyName=$($version.CompanyName), Comments=$($version.Comments)"
