unit frmExportMain;

interface

uses
  Classes, SysUtils, uHighlighterProcs;

procedure ConfigureOpenDialog;

implementation

procedure ConfigureOpenDialog;
var
  fHighlighters: TStringList;
  filter: string;
begin
  fHighlighters := TStringList.Create;
  try
    GetHighlighters(nil, fHighlighters, False);
    filter := GetHighlightersFilter(fHighlighters) + 'All files|*.*|';
  finally
    fHighlighters.Free;
  end;
end;

end.
