unit dmCommands;

interface

uses
  Classes, SysUtils, uHighlighterProcs;

const
  SFilterAllFiles = 'All files (*.*)|*.*|';

procedure SetupFileOpenFilter;

implementation

procedure SetupFileOpenFilter;
var
  fHighlighters: TStringList;
  filter: string;
begin
  fHighlighters := TStringList.Create;
  try
    GetHighlighters(nil, fHighlighters, False);
    filter := GetHighlightersFilter(fHighlighters) + SFilterAllFiles;
  finally
    fHighlighters.Free;
  end;
end;

end.
