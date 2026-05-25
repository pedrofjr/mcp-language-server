{ Hermetic release fixture: socket types exported for blcksock uses-graph tests. }
unit synsock;

interface

type
  TAddrFamily = Integer;
  TMemory = Pointer;
  TLinger = record
    l_onoff: Word;
    l_linger: Word;
  end;
  TVarSin = packed record
    sin_family: Word;
    sin_port: Word;
    sin_addr: LongWord;
    sin_zero: array[0..7] of Byte;
  end;

implementation

end.
