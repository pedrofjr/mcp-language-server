package tools

import "testing"

func TestFindSimilarCode_RejectsSingleTokenOverlapFalsePositive(t *testing.T) {
	src := "ParseJson;"
	query := "ParseJson SaveToDatabase"

	results := FindSimilarCode(src, query, 0.4)
	if len(results) != 0 {
		t.Fatalf("esperado 0 matches quando overlap=1 para query com 2 tokens, obtido %d", len(results))
	}
}

func TestFindSimilarCode_AllowsSingleUsefulTokenQueryWhenLegitMatch(t *testing.T) {
	src := "ParseJson;"
	query := "ParseJson"

	results := FindSimilarCode(src, query, 0.3)
	if len(results) == 0 {
		t.Fatal("esperado match para query curta com 1 token útil")
	}
}

func TestFindSimilarCode_RejectsLowCoverageGenericOverlap(t *testing.T) {
	src := "data value"
	query := "data value customer invoice payment"

	results := FindSimilarCode(src, query, 0.35)
	if len(results) != 0 {
		t.Fatalf("esperado 0 matches para baixa cobertura da query, obtido %d", len(results))
	}
}

func TestFindSimilarCode_FindsNominalBlockRegression(t *testing.T) {
	src := `procedure TFoo.Alpha;
begin
  OpenConnection;
  Execute;
  CloseConnection;
end;

procedure TFoo.Beta;
begin
  OpenConnection;
  RunQuery;
  CloseConnection;
end;`

	query := "OpenConnection Execute CloseConnection"
	results := FindSimilarCode(src, query, 0.3)
	if len(results) == 0 {
		t.Fatal("esperado pelo menos 1 match no cenário nominal")
	}
}