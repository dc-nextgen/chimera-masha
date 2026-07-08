package onboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dc-nextgen/chimera-masha/internal/connector"
)

func TestNewSuggesterNilWhenUnconfigured(t *testing.T) {
	if NewSuggester("", "", "") != nil {
		t.Error("bos config → nil bekleniyordu")
	}
	if NewSuggester("http://x/v1", "", "") != nil {
		t.Error("model bos → nil bekleniyordu")
	}
	if NewSuggester("http://x/v1", "k", "m") == nil {
		t.Error("dolu config → Suggester bekleniyordu")
	}
}

func TestParseAdviceSanitizes(t *testing.T) {
	// markdown citli + gecersiz kind + desteklenmeyen expression + slug'lanacak entity.
	content := "```json\n" + `{"tables":[
      {"table":"dbo.AbpUsers","entity":"Abp Users","kind":"identity","sensitive":true,"note":"kimlik",
       "fields":[{"column":"Email","expression":"mask:email"},{"column":"X","expression":"mask:bogus"},{"column":"","expression":""}]}
    ]}` + "\n```"
	adv, err := parseAdvice(content)
	if err != nil {
		t.Fatalf("parseAdvice: %v", err)
	}
	if len(adv) != 1 {
		t.Fatalf("1 tablo bekleniyordu, %d", len(adv))
	}
	a := adv[0]
	if a.Entity != "abp_users" {
		t.Errorf("entity slug: beklenen abp_users, goren %q", a.Entity)
	}
	if a.Kind != "other" { // "identity" gecersiz → other
		t.Errorf("gecersiz kind → other bekleniyordu, goren %q", a.Kind)
	}
	if !a.Sensitive {
		t.Error("sensitive true bekleniyordu")
	}
	// mask:email kalir, mask:bogus→"" (ama kolon var→kalir), bos-kolon duser.
	if len(a.Fields) != 2 {
		t.Fatalf("2 alan bekleniyordu (bos-kolon duser), goren %d: %+v", len(a.Fields), a.Fields)
	}
	if a.Fields[0].Expression != "mask:email" {
		t.Errorf("mask:email korunmali, goren %q", a.Fields[0].Expression)
	}
	if a.Fields[1].Expression != "" {
		t.Errorf("mask:bogus dusmeli (bos), goren %q", a.Fields[1].Expression)
	}
}

func TestParseAdviceRejectsNonJSON(t *testing.T) {
	if _, err := parseAdvice("uzgunum, yardimci olamam"); err == nil {
		t.Error("JSON'suz icerik → hata bekleniyordu")
	}
}

func TestAdviseCallsLLM(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("beklenmeyen path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("bearer eksik: %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &gotBody)
		resp := map[string]any{"choices": []map[string]any{
			{"message": map[string]any{"content": `{"tables":[{"table":"dbo.Orders","entity":"order","kind":"business","sensitive":false,"note":"siparis","fields":[]}]}`}},
		}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	s := NewSuggester(srv.URL, "secret", "test-model")
	sc := &connector.Schema{Tables: []connector.Table{
		{Schema: "dbo", Name: "Orders", Columns: []connector.Column{{Name: "Id", Type: "int"}}},
	}}
	adv, err := s.Advise(context.Background(), sc)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if len(adv) != 1 || adv[0].Entity != "order" || adv[0].Kind != "business" {
		t.Errorf("beklenmeyen advice: %+v", adv)
	}
	if gotBody["model"] != "test-model" {
		t.Errorf("model gonderilmedi: %v", gotBody["model"])
	}
	// prompt sema YAPISINI icermeli (tablo adi), ama satir/veri gondermemeli.
	msgs, _ := json.Marshal(gotBody["messages"])
	if !strings.Contains(string(msgs), "dbo.Orders") {
		t.Error("prompt sema yapisini icermeli (dbo.Orders)")
	}
}
