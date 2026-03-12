package omnipascal

import "encoding/json"

type request struct {
	Seq       int32  `json:"seq"`
	Type      string `json:"type"`
	Command   string `json:"command"`
	Arguments any    `json:"arguments,omitempty"`
}

type response struct {
	Type       string          `json:"type,omitempty"`
	RequestSeq int32           `json:"request_seq"`
	Success    bool            `json:"success"`
	Body       json.RawMessage `json:"body"`
}

type eventMessage struct {
	Type  string          `json:"type"`
	Event string          `json:"event"`
	Body  json.RawMessage `json:"body"`
}

type Point struct {
	Line   int `json:"line"`
	Offset int `json:"offset"`
}

type TextSpan struct {
	Start Point `json:"start"`
	End   Point `json:"end"`
}

type Diagnostic struct {
	Start    Point  `json:"start"`
	End      Point  `json:"end"`
	Text     string `json:"text"`
	Severity int    `json:"severity"`
}

type DiagnosticsEvent struct {
	File        string       `json:"file"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type ChangeArgs struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	Offset       int    `json:"offset"`
	EndLine      int    `json:"endLine"`
	EndOffset    int    `json:"endOffset"`
	InsertString string `json:"insertString"`
}

type Location struct {
	File  string `json:"file"`
	Start Point  `json:"start"`
	End   Point  `json:"end"`
}

type QuickInfo struct {
	Start         Point  `json:"start"`
	End           Point  `json:"end"`
	DisplayString string `json:"displayString"`
	Documentation string `json:"documentation"`
}

type SignatureHelp struct {
	SelectedItemIndex int             `json:"selectedItemIndex"`
	ArgumentIndex     int             `json:"argumentIndex"`
	Items             []SignatureItem `json:"items"`
}

type SignatureItem struct {
	Name       string               `json:"name"`
	Parameters []SignatureParameter `json:"parameters"`
}

type SignatureParameter struct {
	Label         string `json:"label"`
	Documentation string `json:"documentation"`
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail"`
	Kind           int              `json:"kind"`
	Range          TextSpan         `json:"range"`
	SelectionRange TextSpan         `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children"`
}

type WorkspaceSymbol struct {
	Name          string                  `json:"name"`
	Kind          int                     `json:"kind"`
	ContainerName string                  `json:"containerName"`
	Location      WorkspaceSymbolLocation `json:"location"`
}

type WorkspaceSymbolLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Offset int    `json:"offset"`
}

type ProjectFile struct {
	Name      string `json:"name"`
	TypeLabel string `json:"typeLabel"`
}

type Unit struct {
	Name string `json:"name"`
}

type UsesSectionsResponse struct {
	Sections []string `json:"sections"`
}

type Completion struct {
	Name      string `json:"name"`
	Kind      int    `json:"kind"`
	TypeLabel string `json:"typeLabel"`
	Snippet   string `json:"snippet,omitempty"`
}

type CodeActionList struct {
	CodeActions []CodeActionSummary `json:"CodeActions"`
}

type CodeActionSummary struct {
	Title   string `json:"title"`
	Command string `json:"command"`
}

type CodeActionResponse struct {
	Changes                 []FileChange `json:"changes"`
	ResultingCursorPosition *Point       `json:"resultingCursorPosition,omitempty"`
}

type FileChange struct {
	FilePath string       `json:"filePath"`
	Changes  []TextChange `json:"changes"`
}

type TextChange struct {
	StartLine    int    `json:"startLine"`
	StartColumn  int    `json:"startColumn"`
	EndLine      int    `json:"endLine"`
	EndColumn    int    `json:"endColumn"`
	InsertString string `json:"insertString"`
}
