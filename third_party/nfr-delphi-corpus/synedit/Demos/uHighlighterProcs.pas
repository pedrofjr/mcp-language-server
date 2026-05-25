unit uHighlighterProcs;

interface

uses
  Classes;

function GetHighlighters(AOwner: TComponent; AHighlighters: TStringList;
  AppendToList: boolean): string;
function GetHighlightersFilter(AHighlighters: TStringList): string;

implementation

function GetHighlighters(AOwner: TComponent; AHighlighters: TStringList;
  AppendToList: boolean): string;
begin
  Result := '';
end;

function GetHighlightersFilter(AHighlighters: TStringList): string;
begin
  Result := '';
end;

end.
