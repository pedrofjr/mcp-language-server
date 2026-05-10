package tools

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func topKByScore(results []SimilarBlock, k int) []SimilarBlock {
	if k <= 0 || len(results) == 0 {
		return nil
	}

	ordered := append([]SimilarBlock(nil), results...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Score == ordered[j].Score {
			return ordered[i].StartLine < ordered[j].StartLine
		}
		return ordered[i].Score > ordered[j].Score
	})

	if k > len(ordered) {
		k = len(ordered)
	}
	return ordered[:k]
}

func assertContainsSnippetMarker(t *testing.T, blocks []SimilarBlock, marker string) {
	t.Helper()
	for _, block := range blocks {
		if strings.Contains(block.Snippet, marker) {
			return
		}
	}
	t.Fatalf("esperado marker %q no top-k, mas nenhum snippet correspondeu", marker)
}

func hasAllMarkers(snippet string, markers ...string) bool {
	for _, marker := range markers {
		if !strings.Contains(snippet, marker) {
			return false
		}
	}
	return true
}

func bestScoreWithAllMarkers(blocks []SimilarBlock, markers ...string) float64 {
	bestScore := 0.0
	for _, block := range blocks {
		if hasAllMarkers(block.Snippet, markers...) && block.Score > bestScore {
			bestScore = block.Score
		}
	}
	return bestScore
}

func assertBasicSimilarBlockContract(t *testing.T, blocks []SimilarBlock) {
	t.Helper()
	if len(blocks) == 0 {
		t.Fatal("esperado pelo menos 1 resultado para validar contrato de SimilarBlock")
	}

	for i, block := range blocks {
		if block.StartLine < 0 {
			t.Fatalf("resultado[%d] StartLine invalido: %d", i, block.StartLine)
		}
		if block.EndLine < block.StartLine {
			t.Fatalf("resultado[%d] EndLine invalido: start=%d end=%d", i, block.StartLine, block.EndLine)
		}
		if block.Score <= 0 || block.Score > 1 {
			t.Fatalf("resultado[%d] Score fora de faixa (0,1]: %f", i, block.Score)
		}
		if strings.TrimSpace(block.Snippet) == "" {
			t.Fatalf("resultado[%d] Snippet vazio", i)
		}
	}
}

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

func TestFindSimilarCode_RecognizesStructuralCompatibilityDespiteLexicalRenaming(t *testing.T) {
	src := `procedure TPipeline.Run;
begin
	if CanProceed then
	begin
		OpenChannel;
		RunPipeline;
		CloseChannel;
	end
	else
		RegisterError;
end;`

	query := `if IsReady then
begin
	StartSession;
	ProcessBatch;
	FinishSession;
end
else
	LogFailure;`

	results := FindSimilarCode(src, query, 0.3)
	if len(results) == 0 {
		t.Fatal("esperado match por compatibilidade estrutural mesmo com renomeação lexical relevante")
	}
}

func TestFindSimilarCode_PenalizesLexicalButStructurallyIncompatibleMatch(t *testing.T) {
	src := `procedure TSample.TrapLexicalFalsePositive;
begin
	ValidateItem;
	PersistItem;
	NextItem;
	HasPendingItems;
end;

procedure TSample.RealStructuralCandidate;
begin
	if QueueNotEmpty then
	begin
		while FetchNext do
		begin
			CheckRecord;
			StoreRecord;
		end;
	end;
end;`

	query := `if HasPendingItems then
begin
	while NextItem do
	begin
		ValidateItem;
		PersistItem;
	end;
end;`

	results := FindSimilarCode(src, query, 0.3)
	for _, result := range results {
		if strings.Contains(result.Snippet, "TrapLexicalFalsePositive") {
			t.Fatal("nenhum resultado deve conter trecho lexicalmente parecido porém estruturalmente incompatível (TrapLexicalFalsePositive)")
		}
	}
}

func TestFindSimilarCode_WithOptionsDefaultContract_MatchesLegacyBehavior(t *testing.T) {
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
	legacy := FindSimilarCode(src, query, 0.3)
	withDefaults := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.3,
		WindowLines: 10,
	})

	if !reflect.DeepEqual(legacy, withDefaults) {
		t.Fatalf("esperado caminho com defaults/options equivalente ao legado; legacy=%v options=%v", legacy, withDefaults)
	}
}

func TestFindSimilarCode_WindowLinesSmaller_ChangesScanSliceDeterministically(t *testing.T) {
	src := `alphaone
betatwo
gammathree
deltafour
junkfive
junksix
junkseven
junkeight
junknine
junkten
junkeleven
junktwelve`

	query := "alphaone betatwo gammathree deltafour"

	withWindow10 := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.7,
		WindowLines: 10,
	})

	withWindow4 := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.7,
		WindowLines: 4,
	})

	if len(withWindow4) <= len(withWindow10) {
		t.Fatalf("esperado window_lines menor alterar recorte de varredura e produzir mais matches determinísticos; window4=%d window10=%d", len(withWindow4), len(withWindow10))
	}

	bestTargetScore := bestScoreWithAllMarkers(withWindow4, "alphaone", "betatwo", "gammathree", "deltafour")
	if bestTargetScore < 0.95 {
		t.Fatalf("esperado ao menos 1 match alvo forte contendo alphaone/betatwo/gammathree/deltafour; melhor score=%f", bestTargetScore)
	}
}

func TestFindSimilarCode_RealAuditRecurringInsecurePattern_InTopKWithoutStrictOrder(t *testing.T) {
	src := `procedure TInvoiceRepository.LoadByCustomerUnsafe;
begin
	// UNSAFE_SQL_A
	sql := 'select * from invoices where customer_id=' + customerId;
	LogSQL(sql);
	ExecRawSQL(sql);
	Commit;
end;

procedure TReceiptRepository.LoadByUserUnsafe;
begin
	// UNSAFE_SQL_B
	sql := 'select * from receipts where user_id=' + userId;
	AuditTrail(sql);
	ExecRawSQL(sql);
	Commit;
end;

procedure TInvoiceRepository.LoadSafely;
begin
	stmt := NewStmt('select * from invoices where customer_id=:id');
	stmt.BindInt('id', customerIdInt);
	stmt.Exec;
end;

procedure TCacheWarmup.Warm;
begin
	Ping;
	RefreshCache;
	Sleep(1);
end;`

	query := `sql := 'select * from invoices where customer_id=' + customerId;
LogSQL(sql);
ExecRawSQL(sql);
Commit;`

	results := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.45,
		WindowLines: 8,
	})

	if len(results) == 0 {
		t.Fatal("esperado matches para padrao de SQL inseguro recorrente")
	}

	top := topKByScore(results, 8)
	assertContainsSnippetMarker(t, top, "UNSAFE_SQL_A")
	assertContainsSnippetMarker(t, top, "UNSAFE_SQL_B")
	assertBasicSimilarBlockContract(t, top)
}

func TestFindSimilarCode_RealRefactorEquivalentFlow_StrongLexicalRenameStillMatches(t *testing.T) {
	src := `procedure TSettlementFlow.SyncLedgerWithGateway;
begin
	if EnvelopeReady then
	begin
		while PullNextEnvelope do
		begin
			ValidateEnvelope;
			PublishEnvelope;
		end;
	end
	else
		RegisterEnvelopeFailure;
end;

procedure TSettlementFlow.UnrelatedRoutine;
begin
	ResetStats;
	NotifyIdle;
end;`

	query := `if BatchPrepared then
begin
	while FetchNextBatch do
	begin
		CheckBatch;
		DispatchBatch;
	end;
end
else
	LogBatchFailure;`

	results := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.4,
		WindowLines: 10,
	})

	if len(results) == 0 {
		t.Fatal("esperado match por equivalencia de fluxo mesmo com renomeacao lexical forte")
	}

	top := topKByScore(results, 6)
	assertContainsSnippetMarker(t, top, "ValidateEnvelope")
	assertContainsSnippetMarker(t, top, "PublishEnvelope")

	bestTargetScore := 0.0
	for _, block := range top {
		if hasAllMarkers(block.Snippet, "while PullNextEnvelope do", "ValidateEnvelope") ||
			hasAllMarkers(block.Snippet, "ValidateEnvelope", "PublishEnvelope") {
			if block.Score > bestTargetScore {
				bestTargetScore = block.Score
			}
		}
	}
	if bestTargetScore < 0.55 {
		t.Fatalf("esperado score >= 0.55 para ao menos 1 candidato alvo no top-k; obtido %f", bestTargetScore)
	}
	assertBasicSimilarBlockContract(t, top)
}

func TestFindSimilarCode_RealNegativeMultiBlock_RejectsLexicallySimilarButStructurallyIncompatible(t *testing.T) {
	src := `procedure TImporter.TrueCandidate;
begin
	if HasMoreItems then
	begin
		while NextItem do
		begin
			ValidateItem;
			PersistItem;
		end;
	end;
end;

procedure TImporter.IncompatibleLinear;
begin
	ValidateItem;
	PersistItem;
	NextItem;
	HasMoreItems;
end;

procedure TImporter.IncompatibleTryCase;
begin
	try
		ValidateItem;
		PersistItem;
		case CurrentState of
			1: NextItem;
		end;
	except
		On E: Exception do PersistItem;
	end;
end;`

	query := `if HasMoreItems then
begin
	while NextItem do
	begin
		ValidateItem;
		PersistItem;
	end;
end;`

	results := FindSimilarCodeWithOptions(src, query, FindSimilarCodeOptions{
		Threshold:   0.42,
		WindowLines: 8,
	})

	if len(results) == 0 {
		t.Fatal("esperado ao menos 1 match estruturalmente compativel")
	}

	top := topKByScore(results, 8)
	assertContainsSnippetMarker(t, top, "TrueCandidate")

	validTargetCount := 0
	for _, block := range top {
		if hasAllMarkers(block.Snippet, "if HasMoreItems then", "while NextItem do", "ValidateItem", "PersistItem") {
			validTargetCount++
			continue
		}

		// Snippets cruzando fronteira podem carregar nomes de rotina; bloqueamos apenas candidatos fortes sem estrutura.
		if (strings.Contains(block.Snippet, "IncompatibleLinear") || strings.Contains(block.Snippet, "IncompatibleTryCase")) &&
			block.Score >= 0.55 {
			t.Fatalf("bloco incompatível apareceu como candidato forte (score=%.3f) sem estrutura alvo: %q", block.Score, block.Snippet)
		}
	}

	if validTargetCount == 0 {
		t.Fatal("esperado ao menos 1 candidato valido com marcadores estruturais if/while/validate/persist no top-k")
	}

	assertBasicSimilarBlockContract(t, top)
}
