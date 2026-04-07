[Setup]
AppName=Odoo Print Agent
AppVersion=1.0.0
DefaultDirName={pf}\OdooPrintAgent
DefaultGroupName=Odoo Print Agent
OutputBaseFilename=OdooPrintAgentInstaller
Compression=lzma
SolidCompression=yes

[Tasks]
Name: autostart; Description: Start agent on login; Flags: unchecked
Name: desktopicon; Description: Create a desktop icon; Flags: unchecked

[Files]
Source: "..\\..\\dist\\windows\\odoo-print-agent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\\..\\config.json"; DestDir: "{app}"; Flags: onlyifdoesntexist

[Icons]
Name: "{group}\Odoo Print Agent"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{app}\config.json"""
Name: "{userdesktop}\Odoo Print Agent"; Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{app}\config.json"""; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "OdooPrintAgent"; ValueData: """{app}\odoo-print-agent.exe"" run --config ""{app}\config.json"""; Tasks: autostart

[Run]
Filename: "{app}\odoo-print-agent.exe"; Parameters: "run --config ""{app}\config.json"""; Description: "Start Odoo Print Agent"; Flags: nowait postinstall skipifsilent

[Code]
var
  OdooURLPage: TInputQueryWizardPage;
  ApiKeyPage: TInputQueryWizardPage;

procedure InitializeWizard;
begin
  OdooURLPage := CreateInputQueryPage(wpSelectTasks, 'Odoo Connection', 'Enter your Odoo URL', '');
  OdooURLPage.Add('Odoo URL:', False);
  OdooURLPage.Values[0] := '';

  ApiKeyPage := CreateInputQueryPage(OdooURLPage.ID, 'Agent Authentication', 'Enter your API key', '');
  ApiKeyPage.Add('API Key:', False);
end;

function ConfigureAndValidate: Boolean;
var
  ResultCode: Integer;
  Args: string;
begin
  Args := 'configure --config "' + ExpandConstant('{app}\config.json') + '" --odoo-url "' + OdooURLPage.Values[0] + '" --api-key "' + ApiKeyPage.Values[0] + '"';
  Result := Exec(ExpandConstant('{app}\odoo-print-agent.exe'), Args, ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode) and (ResultCode = 0);
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = ApiKeyPage.ID then
  begin
    if not ConfigureAndValidate then
    begin
      MsgBox('Validation failed. Check Odoo URL/API key and try again.', mbError, MB_OK);
      Result := False;
    end;
  end;
end;
