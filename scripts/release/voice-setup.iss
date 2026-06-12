; Inno Setup script for Voice Windows desktop (WinSparkle-compatible installer).
#ifndef AppVersion
  #define AppVersion "1.0.0"
#endif
#ifndef BuildDir
  #define BuildDir "..\..\src\frontend\build\windows\x64\runner\Release"
#endif

[Setup]
AppName=Voice
AppVersion={#AppVersion}
DefaultDirName={autopf}\Voice
DefaultGroupName=Voice
OutputBaseFilename=VoiceSetup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
ArchitecturesInstallIn64BitMode=x64

[Files]
Source: "{#BuildDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs

[Icons]
Name: "{group}\Voice"; Filename: "{app}\voice_frontend.exe"
