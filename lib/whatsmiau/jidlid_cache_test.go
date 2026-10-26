package whatsmiau

import (
	"context"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

func TestWithJidLidCacheReaproveitaCacheExistente(t *testing.T) {
	ctx := WithJidLidCache(context.Background())
	mesmo := WithJidLidCache(ctx)

	if ctx.Value(jidLidCacheKey{}) != mesmo.Value(jidLidCacheKey{}) {
		t.Fatal("chamar WithJidLidCache duas vezes deveria manter o mesmo cache")
	}
}

func TestGetJidLidRespondePeloCacheSemTocarNoStore(t *testing.T) {
	ctx := WithJidLidCache(context.Background())
	cache, ok := ctx.Value(jidLidCacheKey{}).(*jidLidCache)
	if !ok {
		t.Fatal("o contexto deveria carregar o cache")
	}

	jid := types.JID{User: "5585999999999", Server: types.DefaultUserServer}
	cache.entries[jid] = [2]string{"jid-guardado", "lid-guardado"}

	// A instancia nao existe: sem cache, extractJidLid devolveria o proprio JID.
	// Receber o valor guardado prova que a consulta foi evitada.
	s := novoParaTeste()
	gotJid, gotLid := s.GetJidLid(ctx, "instancia-inexistente", jid)

	if gotJid != "jid-guardado" || gotLid != "lid-guardado" {
		t.Fatalf("esperava os valores do cache, veio %q e %q", gotJid, gotLid)
	}
}

func TestGetJidLidSemCacheContinuaFuncionando(t *testing.T) {
	jid := types.JID{User: "5585999999999", Server: types.DefaultUserServer}

	s := novoParaTeste()
	gotJid, _ := s.GetJidLid(context.Background(), "instancia-inexistente", jid)

	if gotJid != jid.ToNonAD().String() {
		t.Fatalf("sem cache no contexto o comportamento deveria ser o de antes, veio %q", gotJid)
	}
}

// novoParaTeste monta o minimo necessario para exercitar GetJidLid: o mapa de
// clients, que extractJidLid consulta antes de qualquer coisa.
func novoParaTeste() *Whatsmiau {
	return &Whatsmiau{clients: xsync.NewMap[string, *whatsmeow.Client]()}
}
