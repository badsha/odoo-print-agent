[Setup]
AppName=Odoo Print Agent
AppVersion=1.0.0
DefaultDirName={pf}\OdooPrintAgent
DefaultGroupName=Odoo Print Agent
OutputBaseFilename=OdooPrintAgentInstaller
Compression=lzma
SolidCompression=yes

[Tasks]
Name: service; Description: Install as Windows Service (recommended); Flags: checkedonce
Name: autostart; Description: Start agent on login; Flags: unchecked
Name: desktopicon; Description: Create a desktop icon; Flags: unchecked

[Dirs]
Name: "{commonappdata}\OdooPrintAgent"
Name: "{commonappdata}\OdooPrintAgent\logs"

[Files]
Source: "..\\..\\dist\\windows\\odoo-print-agent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\\..\\config.json"; DestDir: "{commonappdata}\OdooPrintAgent"; Flags: onlyifdoesntexist
Source: "..\\..\\dist\\windows\\SumatraPDF.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\\..\\dist\\windows\\nssm.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Odoo Print Agent"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{commonappdata}\OdooPrintAgent\config.json"""
Name: "{userdesktop}\Odoo Print Agent"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{commonappdata}\OdooPrintAgent\config.json"""; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "OdooPrintAgent"; ValueData: """{app}\odoo-print-agent.exe"" run --config ""{commonappdata}\OdooPrintAgent\config.json"""; Tasks: autostart

[Run]
Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{commonappdata}\OdooPrintAgent\config.json"""; Description: "Start Odoo Print Agent"; Flags: nowait postinstall skipifsilent; Tasks: autostart

[Code]
var
  OdooURLPage: TInputQueryWizardPage;
  ApiKeyPage: TInputQueryWizardPage;
  PrinterPage: TWizardPage;
  PrinterCombo: TNewComboBox;

procedure InitializeWizard;
var
  ResultCode: Integer;
  OutFile: string;
  Contents: string;
  Lines: TArrayOfString;
  I: Integer;
begin
  OdooURLPage := CreateInputQueryPage(wpSelectTasks, 'Odoo Connection', 'Enter your Odoo URL', '');
  OdooURLPage.Add('Odoo URL:', False);
  OdooURLPage.Values[0] := '';

  PrinterPage := CreateCustomPage(OdooURLPage.ID, 'Printer Selection', 'Choose the Windows printer to use for this agent');
  PrinterCombo := TNewComboBox.Create(PrinterPage);
  PrinterCombo.Parent := PrinterPage.Surface;
  PrinterCombo.Left := ScaleX(0);
  PrinterCombo.Top := ScaleY(8);
  PrinterCombo.Width := PrinterPage.SurfaceWidth;
  PrinterCombo.Style := csDropDownList;

  OutFile := ExpandConstant('{tmp}\odoo_print_agent_printers.txt');
  Exec('powershell.exe',
    '-NoProfile -ExecutionPolicy Bypass -Command "Get-Printer | Select-Object -ExpandProperty Name | Out-File -Encoding UTF8 ''' + OutFile + '''"',
    '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  if LoadStringFromFile(OutFile, Contents) then
  begin
    Lines := SplitString(Contents, #13#10);
    for I := 0 to GetArrayLength(Lines) - 1 do
    begin
      if Trim(Lines[I]) <> '' then
        PrinterCombo.Items.Add(Trim(Lines[I]));
    end;
  end;
  if PrinterCombo.Items.Count > 0 then
    PrinterCombo.ItemIndex := 0;

  ApiKeyPage := CreateInputQueryPage(PrinterPage.ID, 'Agent Authentication', 'Enter your API key', '');
  ApiKeyPage.Add('API Key:', False);
end;

function ConfigureAndValidate: Boolean;
var
  ResultCode: Integer;
  Args: string;
begin
  Args :=
    'setup --config "' + ExpandConstant('{commonappdata}\OdooPrintAgent\config.json') + '"' +
    ' --odoo-url "' + OdooURLPage.Values[0] + '"' +
    ' --api-key "' + ApiKeyPage.Values[0] + '"' +
    ' --os-printer-name "' + PrinterCombo.Text + '"' +
    ' --log-file "' + ExpandConstant('{commonappdata}\OdooPrintAgent\logs\agent.jsonl') + '"' +
    ' --log-level info' +
    ' --test-print';
  Result := Exec(ExpandConstant('{app}\odoo-print-agent.exe'), Args, ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = PrinterPage.ID then
  begin
    if (PrinterCombo.Items.Count = 0) or (Trim(PrinterCombo.Text) = '') then
    begin
      MsgBox('No Windows printers detected. Install a printer and try again.', mbError, MB_OK);
      Result := False;
    end;
  end;
  if CurPageID = ApiKeyPage.ID then
  begin
    if not ConfigureAndValidate then
    begin
      MsgBox('Setup failed. Check Odoo URL/API key/printer selection and try again.', mbError, MB_OK);
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
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppRotateOnline 1', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
      Exec(ExpandConstant('{app}\nssm.exe'), 'set OdooPrintAgent AppRotateBytes 10485760', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
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
