unit SymbolMutateHarness;

implementation

procedure TMutate.Target;
begin
  Writeln('seed');
end;

procedure DeleteMe;
begin
  Writeln('delete-me');
end;

end.
