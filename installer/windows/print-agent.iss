; Odoo Print Agent — merchant one-click Windows installer
; Build with: installer\windows\build.ps1

[Setup]
AppId={{8F3C2A91-6B47-4D1E-9E55-2C8A0F7B1D33}}
AppName=Odoo Print Agent
AppVersion=1.0.1
AppPublisher=LogicLayer
DefaultDirName={autopf}\OdooPrintAgent
DefaultGroupName=Odoo Print Agent
DisableProgramGroupPage=yes
OutputDir=..\..\dist\windows
OutputBaseFilename=OdooPrintAgentSetup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=
UninstallDisplayIcon={app}\odoo-print-agent.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: service; Description: "Install as Windows Service (recommended — keeps printing after you close Setup)"; Flags: checkedonce
Name: desktopicon; Description: "Create a desktop shortcut to open Setup again"; Flags: unchecked

[Dirs]
Name: "{commonappdata}\OdooPrintAgent"
Name: "{commonappdata}\OdooPrintAgent\logs"
Name: "{commonappdata}\OdooPrintAgent\spool"

[Files]
Source: "..\..\dist\windows\odoo-print-agent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\windows\SumatraPDF.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\dist\windows\nssm.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\config.example.json"; DestDir: "{commonappdata}\OdooPrintAgent"; DestName: "config.json"; Flags: onlyifdoesntexist ignoreversion

[Icons]
Name: "{group}\Odoo Print Agent Setup"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "ui --config ""{commonappdata}\OdooPrintAgent\config.json"""
Name: "{group}\Uninstall Odoo Print Agent"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Odoo Print Agent Setup"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "ui --config ""{commonappdata}\OdooPrintAgent\config.json"""; Tasks: desktopicon

[Code]
var
  ConnPage: TInputQueryWizardPage;
  PrinterPage: TWizardPage;
  PrinterChecks: array of TNewCheckBox;
  PrinterNames: TArrayOfString;

procedure LoadWindowsPrinters;
var
  ResultCode: Integer;
  OutFile: string;
  Lines: TArrayOfString;
  I, Count: Integer;
  Y: Integer;
  NameStr: string;
begin
  OutFile := ExpandConstant('{tmp}\odoo_print_agent_printers.txt');
  Exec('powershell.exe',
    '-NoProfile -ExecutionPolicy Bypass -Command "Get-Printer | Select-Object -ExpandProperty Name | Out-File -Encoding ascii ''' + OutFile + '''"',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);

  Count := 0;
  if not LoadStringsFromFile(OutFile, Lines) then
    Exit;

  for I := 0 to GetArrayLength(Lines) - 1 do
  begin
    NameStr := Trim(Lines[I]);
    if NameStr <> '' then
      Inc(Count);
  end;

  SetArrayLength(PrinterNames, Count);
  SetArrayLength(PrinterChecks, Count);
  Count := 0;
  Y := ScaleY(8);
  for I := 0 to GetArrayLength(Lines) - 1 do
  begin
    NameStr := Trim(Lines[I]);
    if NameStr = '' then
      Continue;
    PrinterNames[Count] := NameStr;
    PrinterChecks[Count] := TNewCheckBox.Create(PrinterPage);
    PrinterChecks[Count].Parent := PrinterPage.Surface;
    PrinterChecks[Count].Left := ScaleX(0);
    PrinterChecks[Count].Top := Y;
    PrinterChecks[Count].Width := PrinterPage.SurfaceWidth;
    PrinterChecks[Count].Caption := NameStr;
    if (Pos('OneNote', NameStr) = 0) and
       (Pos('XPS', NameStr) = 0) and
       (Pos('PDF', NameStr) = 0) and
       (Pos('Fax', NameStr) = 0) then
      PrinterChecks[Count].Checked := True;
    Y := Y + ScaleY(22);
    Inc(Count);
  end;
end;

function SelectedPrinterNames: string;
var
  I: Integer;
  Acc: string;
begin
  Acc := '';
  for I := 0 to GetArrayLength(PrinterChecks) - 1 do
  begin
    if PrinterChecks[I].Checked then
    begin
      if Acc <> '' then
        Acc := Acc + '|';
      Acc := Acc + PrinterNames[I];
    end;
  end;
  Result := Acc;
end;

procedure InitializeWizard;
begin
  ConnPage := CreateInputQueryPage(wpSelectTasks,
    'Connect to Odoo',
    'Paste the details from Odoo Printing Setup (no Command Prompt needed).',
    '');
  ConnPage.Add('Odoo address (example http://127.0.0.1:8069):', False);
  ConnPage.Add('Database name (required if Odoo has more than one database):', False);
  ConnPage.Add('API key (from Printing → Configuration → Printing Setup):', False);
  ConnPage.Values[0] := 'http://127.0.0.1:8069';
  ConnPage.Values[1] := '';
  ConnPage.Values[2] := '';

  PrinterPage := CreateCustomPage(ConnPage.ID,
    'Which printers?',
    'Tick the Windows printers this PC should use with Print Master.');
  LoadWindowsPrinters;
end;

function ConfigureAgent: Boolean;
var
  ResultCode: Integer;
  Args: string;
  Printers: string;
begin
  Printers := SelectedPrinterNames;
  Args :=
    'setup --config "' + ExpandConstant('{commonappdata}\OdooPrintAgent\config.json') + '"' +
    ' --odoo-url "' + Trim(ConnPage.Values[0]) + '"' +
    ' --database "' + Trim(ConnPage.Values[1]) + '"' +
    ' --api-key "' + Trim(ConnPage.Values[2]) + '"' +
    ' --os-printer-names "' + Printers + '"' +
    ' --sumatra-pdf-path "' + ExpandConstant('{app}\SumatraPDF.exe') + '"' +
    ' --log-file "' + ExpandConstant('{commonappdata}\OdooPrintAgent\logs\agent.jsonl') + '"' +
    ' --spool-dir "' + ExpandConstant('{commonappdata}\OdooPrintAgent\spool') + '"' +
    ' --log-level info';
  Result := Exec(ExpandConstant('{app}\odoo-print-agent.exe'), Args, ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = ConnPage.ID then
  begin
    if Trim(ConnPage.Values[0]) = '' then
    begin
      MsgBox('Please enter your Odoo address.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if Trim(ConnPage.Values[2]) = '' then
    begin
      MsgBox('Please paste the API key from Odoo Printing Setup.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
  end;
  if CurPageID = PrinterPage.ID then
  begin
    if GetArrayLength(PrinterChecks) = 0 then
    begin
      MsgBox('No Windows printers were found. Install a printer in Windows, then run Setup again.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if SelectedPrinterNames = '' then
    begin
      MsgBox('Please tick at least one printer.', mbError, MB_OK);
      Result := False;
      Exit;
    end;
    if not ConfigureAgent then
    begin
      MsgBox('Could not connect to Odoo. Check the address, database name, and API key, then try again.', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
  ServiceArgs: string;
begin
  if CurStep = ssPostInstall then
  begin
    if WizardIsTaskSelected('service') then
    begin
      ServiceArgs :=
        'install OdooPrintAgent "' + ExpandConstant('{app}\odoo-print-agent.exe') + '" run --config "' +
        ExpandConstant('{commonappdata}\OdooPrintAgent\config.json') + '"';
      Exec(ExpandConstant('{app}\nssm.exe'), ServiceArgs, ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppDirectory "' + ExpandConstant('{app}') + '"', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppStdout "' + ExpandConstant('{commonappdata}\OdooPrintAgent\logs\agent.log') + '"', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppStderr "' + ExpandConstant('{commonappdata}\OdooPrintAgent\logs\agent.log') + '"', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppRotateFiles 1', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppRestartDelay 2000', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'start OdooPrintAgent', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
    end;
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  ResultCode: Integer;
begin
  if CurUninstallStep = usUninstall then
  begin
    if FileExists(ExpandConstant('{app}\nssm.exe')) then
    begin
      Exec(ExpandConstant('{app}\nssm.exe'), 'stop OdooPrintAgent', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'remove OdooPrintAgent confirm', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
    end;
  end;
end;
