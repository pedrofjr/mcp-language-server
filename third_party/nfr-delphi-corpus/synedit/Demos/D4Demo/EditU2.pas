unit EditU2;

interface

uses
  Classes, SysUtils, uHighlighterProcs;

procedure ReloadHighlighterList;

implementation

procedure ReloadHighlighterList;
var
  fHighlighters: TStringList;
  s: string;
begin
  fHighlighters := TStringList.Create;
  try
    GetHighlighters(nil, fHighlighters, False);
    s := GetHighlightersFilter(fHighlighters);
    if (s <> '') and (s[Length(s)] <> '|') then
      s := s + '|';
    s := s + 'All files (*.*)|*.*';
  finally
    fHighlighters.Free;
  end;
end;

end.
