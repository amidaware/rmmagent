#define MyAppName "Tactical RMM Agent"
#define MyAppVersion "2.6.1"
#define MyAppPublisher "AmidaWare Inc"
#define MyAppURL "https://github.com/amidaware"
#define MyAppExeName "tacticalrmm.exe"
#define MESHEXE "meshagent.exe"
#define MESHDIR "{sd}\Program Files\Mesh Agent"

[Setup]
AppId={{0D34D278-5FAF-4159-A4A0-4E2D2C08139D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName="{sd}\Program Files\TacticalAgent"
DisableDirPage=yes
SetupLogging=yes
DisableProgramGroupPage=yes
SetupIconFile=C:\Users\Public\Documents\agent\build\onit.ico
WizardSmallImageFile=C:\Users\Public\Documents\agent\build\onit.bmp
UninstallDisplayIcon={app}\{#MyAppExeName}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
RestartApplications=no
CloseApplications=no
MinVersion=6.0

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "C:\Users\Public\Documents\agent\tacticalrmm.exe"; DestDir: "{app}"; Flags: ignoreversion;

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent runascurrentuser

[UninstallRun]
Filename: "{app}\{#MyAppExeName}"; Parameters: "-m cleanup"; RunOnceId: "cleanuprm";
Filename: "{cmd}"; Parameters: "/c taskkill /F /IM tacticalrmm.exe"; RunOnceId: "killtacrmm";
Filename: "{app}\{#MESHEXE}"; Parameters: "-fulluninstall"; RunOnceId: "meshrm";

[UninstallDelete]
Type: filesandordirs; Name: "{app}";
Type: filesandordirs; Name: "{#MESHDIR}";

[Code]
function InitializeSetup(): boolean;
var
  ResultCode: Integer;
begin
  Exec('cmd.exe', '/c ping 127.0.0.1 -n 2 && net stop tacticalrpc', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('Stop tacticalrpc: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c net stop tacticalagent', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('Stop tacticalagent: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c ping 127.0.0.1 -n 2 && net stop tacticalrmm', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('Stop tacticalrmm: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c taskkill /F /IM tacticalrmm.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('taskkill: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c sc delete tacticalagent', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('delete tacticalagent: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c sc delete tacticalrpc', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('delete tacticalrpc: ' + IntToStr(ResultCode));

  Result := True;
end;

procedure DeinitializeSetup();
var
  ResultCode: Integer;
  WorkingDir:   String;
begin

  WorkingDir := ExpandConstant('{sd}\Program Files\TacticalAgent');
  Exec('cmd.exe', ' /c tacticalrmm.exe -m installsvc', WorkingDir, SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('install service: ' + IntToStr(ResultCode));

  Exec('cmd.exe', '/c net start tacticalrmm', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('Start tacticalrmm: ' + IntToStr(ResultCode));
end;

function InitializeUninstall(): Boolean;
var
  ResultCode: Integer;
begin
  Exec('cmd.exe', '/c ping 127.0.0.1 -n 2 && net stop tacticalrmm', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Exec('cmd.exe', '/c taskkill /F /IM tacticalrmm.exe', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

  Exec('cmd.exe', '/c sc delete tacticalrmm', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Log('delete tacticalrmm: ' + IntToStr(ResultCode));

  Result := True;
end;
